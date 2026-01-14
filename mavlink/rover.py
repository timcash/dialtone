import time
import logging
import argparse
import sys
import json
import select
import asyncio
from typing import List, Dict, Any, Optional
from threading import Thread
from datetime import datetime
from pymavlink import mavutil
import nats
from nats.errors import ConnectionClosedError, TimeoutError, NoServersError

# ArduRover mode mapping (different from ArduCopter)
ROVER_MODES = {
    0: "MANUAL",
    1: "ACRO", 
    3: "STEERING",
    4: "HOLD",
    5: "LOITER",
    6: "FOLLOW",
    7: "SIMPLE",
    8: "DOCK",
    9: "CIRCLE",
    10: "AUTO",
    11: "RTL",
    12: "SMART_RTL",
    15: "GUIDED",
    16: "INITIALIZING"
}

# Configure logging with custom format [time_UTC level module function line] message
class CustomFormatter(logging.Formatter):
    def format(self, record):
        timestamp = datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%S,%f")[:-3] + "Z"
        level = record.levelname
        module = record.module
        func = record.funcName
        line = record.lineno
        message = record.getMessage()
        return f"[{timestamp} {level} {module}.py {func} {line}] {message}"

# Set up logger
logger = logging.getLogger('rover')
logger.setLevel(logging.INFO)

# Create console handler with custom formatter
console_handler = logging.StreamHandler()
console_handler.setFormatter(CustomFormatter())
logger.addHandler(console_handler)
logger.addHandler(console_handler)

# Create file handler with custom formatter
file_handler = logging.FileHandler('rover_operations.log')
file_handler.setFormatter(CustomFormatter())
logger.addHandler(file_handler)

class GPS:
    def __init__(self):
        self.latitude: float = 0.0
        self.longitude: float = 0.0
        self.altitude: float = 0.0
        self.fix_type: int = 0
        self.satellites: int = 0

class EKF:
    def __init__(self):
        self.status: str = "INITIALIZING"
        self.healthy: bool = False

class State:
    def __init__(self):
        self.gps: GPS = GPS()
        self.ekf: EKF = EKF()
        self.speed: float = 0.0
        self.heading: float = 0.0
        self.armed: bool = False
        self.mode: str = "UNKNOWN"
        self.battery_voltage: float = 0.0
        self.battery_remaining: int = 0
        self.roll: float = 0.0
        self.pitch: float = 0.0
        self.yaw: float = 0.0
        self.last_update: float = 0.0

    def to_dict(self):
        return {
            "gps": {
                "latitude": self.gps.latitude,
                "longitude": self.gps.longitude,
                "altitude": self.gps.altitude,
                "fix_type": self.gps.fix_type,
                "satellites": self.gps.satellites
            },
            "ekf": {
                "status": self.ekf.status,
                "healthy": self.ekf.healthy
            },
            "speed": self.speed,
            "heading": self.heading,
            "armed": self.armed,
            "mode": self.mode,
            "battery_voltage": self.battery_voltage,
            "battery_remaining": self.battery_remaining,
            "attitude": {
                "roll": self.roll,
                "pitch": self.pitch,
                "yaw": self.yaw
            },
            "last_update": self.last_update
        }

class Rover:
    def __init__(self, connection_string):
        self.connection_string = connection_string
        self.master = None
        self.target_system = None
        self.target_component = None
        self.last_heartbeat = None
        self.mock_mode = False
        self.mock_start_time = 0
        self.state = State()
        self.telemetry = self.state.to_dict() # For backward compatibility if needed

    def __enter__(self):
        self.connect()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    def connect(self):
        """Connects to the rover and returns True if successful, False otherwise."""
        logger.info(f"Attempting to connect to rover at {self.connection_string}", extra={'system': 'CONNECTION'})
        
        try:
            # For serial connections, set appropriate baud rate (57600 is standard for ArduPilot)
            # mavutil will auto-detect the connection type from the connection string
            self.master = mavutil.mavlink_connection(self.connection_string, baud=57600)
            if not self.master:
                logger.error("Failed to create MAVLink connection object", extra={'system': 'CONNECTION'})
                return False
            
            # Wait for the first heartbeat to confirm connection
            logger.info("Waiting for heartbeat...", extra={'system': 'CONNECTION'})
            heartbeat = self.master.wait_heartbeat(timeout=10)
            
            if heartbeat is None:
                logger.error("No heartbeat received - connection failed", extra={'system': 'CONNECTION'})
                return False
            
            # Use the first heartbeat we received to set target system and component
            target_system = heartbeat.get_srcSystem()
            target_component = heartbeat.get_srcComponent()
            
            logger.info(f"Found heartbeat from System {target_system} Component {target_component} Type {heartbeat.type}", 
                       extra={'system': 'CONNECTION'})
            
            # If this is not the main flight controller, look for it briefly
            if heartbeat.type != 10:  # MAV_TYPE_GROUND_ROVER
                logger.info("First heartbeat not from main flight controller, looking for GROUND_ROVER...", extra={'system': 'CONNECTION'})
                
                # Look for the main flight controller with short timeout
                for _ in range(5):  # Try for 5 seconds max
                    msg = self.master.recv_match(type='HEARTBEAT', blocking=True, timeout=1)
                    if msg and msg.type == 10:  # MAV_TYPE_GROUND_ROVER
                        target_system = msg.get_srcSystem()
                        target_component = msg.get_srcComponent()
                        logger.info(f"Found main flight controller: System {target_system} Component {target_component}", 
                                   extra={'system': 'CONNECTION'})
                        break
                    elif msg:
                        # Log other heartbeats for debugging
                        logger.debug(f"Found heartbeat from System {msg.get_srcSystem()} Component {msg.get_srcComponent()} Type {msg.type}", 
                                   extra={'system': 'CONNECTION'})
                
                if target_system == heartbeat.get_srcSystem():  # We didn't find a GROUND_ROVER
                    logger.error("Could not find GROUND_ROVER type - connection failed", extra={'system': 'CONNECTION'})
                    return False
            
            self.target_system = target_system
            self.target_component = target_component
            
            # Update the master's target system and component
            self.master.target_system = target_system
            self.master.target_component = target_component
            
            logger.info(f"Connected to system {self.target_system} component {self.target_component}", 
                       extra={'system': 'CONNECTION'})
            
            # Log initial heartbeat
            self._log_heartbeat()
            
            # Log comprehensive system information on startup
            self._log_system_startup_info()
            
            return True
            
        except Exception as e:
            logger.error(f"Failed to connect to rover: {e}", extra={'system': 'CONNECTION'})
            return False

    def close(self):
        """Closes the connection to the rover."""
        if self.master:
            self.master.close()
            logger.info("Connection closed", extra={'system': 'CONNECTION'})

    def _log_heartbeat(self):
        """Log current heartbeat information from the main flight controller."""
        try:
            # Look for heartbeat from the main flight controller with short timeout
            for _ in range(3):  # Try for 3 seconds max
                msg = self.master.recv_match(type='HEARTBEAT', blocking=True, timeout=1)
                if msg and msg.get_srcSystem() == self.target_system and msg.get_srcComponent() == self.target_component:
                    self.last_heartbeat = msg
                    mode_name = self.get_rover_mode_name(msg.custom_mode)
                    is_armed = bool(msg.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED)
                    armed_status = "ARMED" if is_armed else "DISARMED"
                    
                    logger.info(f"Heartbeat - Mode: {mode_name} ({msg.custom_mode}) | Armed: {armed_status} | Type: {msg.type}", 
                               extra={'system': f'SYS{msg.get_srcSystem()}_COMP{msg.get_srcComponent()}'})
                    return
                elif msg:
                    # Log other heartbeats at debug level
                    logger.debug(f"Non-target heartbeat from SYS{msg.get_srcSystem()}_COMP{msg.get_srcComponent()} Type: {msg.type}", 
                               extra={'system': 'HEARTBEAT'})
            
            logger.error("Could not get heartbeat from main flight controller - connection failed", extra={'system': 'HEARTBEAT'})
            return False
        except Exception as e:
            logger.warning(f"Could not get heartbeat: {e}", extra={'system': 'HEARTBEAT'})

    def _log_system_startup_info(self):
        """Log comprehensive system information on startup."""
        logger.info("=== SYSTEM STARTUP INFORMATION ===", extra={'system': 'STARTUP'})
        
        try:
            # Request autopilot version information
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_AUTOPILOT_VERSION,
                0, 0, 0, 0, 0, 0)
            
            # Request system status
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_SYS_STATUS,
                0, 0, 0, 0, 0, 0)
            
            # Request GPS raw data
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_GPS_RAW_INT,
                0, 0, 0, 0, 0, 0)
            
            # Request attitude information
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_ATTITUDE,
                0, 0, 0, 0, 0, 0)
            
            # Request global position
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_GLOBAL_POSITION_INT,
                0, 0, 0, 0, 0, 0)
            
            # Collect messages for 5 seconds
            start_time = time.time()
            autopilot_version_received = False
            sys_status_received = False
            gps_raw_received = False
            attitude_received = False
            global_position_received = False
            
            while time.time() - start_time < 5:
                msg = self.master.recv_match(blocking=False)
                if msg:
                    msg_type = msg.get_type()
                    
                    if msg_type == 'AUTOPILOT_VERSION' and not autopilot_version_received:
                        self._log_autopilot_version(msg)
                        autopilot_version_received = True
                    
                    elif msg_type == 'SYS_STATUS' and not sys_status_received:
                        self._log_system_status(msg)
                        sys_status_received = True
                    
                    elif msg_type == 'GPS_RAW_INT' and not gps_raw_received:
                        self._log_gps_status(msg)
                        gps_raw_received = True
                    
                    elif msg_type == 'ATTITUDE' and not attitude_received:
                        self._log_attitude(msg)
                        attitude_received = True
                    
                    elif msg_type == 'GLOBAL_POSITION_INT' and not global_position_received:
                        self._log_global_position(msg)
                        global_position_received = True
                
                time.sleep(0.1)
            
            # Log any missing information
            if not autopilot_version_received:
                logger.warning("AUTOPILOT_VERSION not received", extra={'system': 'STARTUP'})
            if not sys_status_received:
                logger.warning("SYS_STATUS not received", extra={'system': 'STARTUP'})
            if not gps_raw_received:
                logger.warning("GPS_RAW_INT not received", extra={'system': 'STARTUP'})
            if not attitude_received:
                logger.warning("ATTITUDE not received", extra={'system': 'STARTUP'})
            if not global_position_received:
                logger.warning("GLOBAL_POSITION_INT not received", extra={'system': 'STARTUP'})
            
            logger.info("=== END SYSTEM STARTUP INFORMATION ===", extra={'system': 'STARTUP'})
            
        except Exception as e:
            logger.error(f"Error collecting startup information: {e}", extra={'system': 'STARTUP'})

    def _log_autopilot_version(self, msg):
        """Log autopilot version information."""
        logger.info("--- AUTOPILOT VERSION ---", extra={'system': 'STARTUP'})
        logger.info(f"Flight Software Version: {msg.flight_sw_version}", extra={'system': 'STARTUP'})
        logger.info(f"Middleware Version: {msg.middleware_sw_version}", extra={'system': 'STARTUP'})
        logger.info(f"OS Software Version: {msg.os_sw_version}", extra={'system': 'STARTUP'})
        logger.info(f"Board Version: {msg.board_version}", extra={'system': 'STARTUP'})
        logger.info(f"Vendor ID: {msg.vendor_id}", extra={'system': 'STARTUP'})
        logger.info(f"Product ID: {msg.product_id}", extra={'system': 'STARTUP'})
        logger.info(f"UID: {msg.uid}", extra={'system': 'STARTUP'})
        logger.info(f"Capabilities: {msg.capabilities}", extra={'system': 'STARTUP'})

    def _log_system_status(self, msg):
        """Log system status information."""
        logger.info("--- SYSTEM STATUS ---", extra={'system': 'STARTUP'})
        logger.info(f"Sensors Present: 0x{msg.onboard_control_sensors_present:08X}", extra={'system': 'STARTUP'})
        logger.info(f"Sensors Enabled: 0x{msg.onboard_control_sensors_enabled:08X}", extra={'system': 'STARTUP'})
        logger.info(f"Sensors Health: 0x{msg.onboard_control_sensors_health:08X}", extra={'system': 'STARTUP'})
        logger.info(f"CPU Load: {msg.load / 10.0:.1f}%", extra={'system': 'STARTUP'})
        logger.info(f"Battery Voltage: {msg.voltage_battery / 1000.0:.2f}V", extra={'system': 'STARTUP'})
        logger.info(f"Battery Current: {msg.current_battery / 100.0:.2f}A", extra={'system': 'STARTUP'})
        logger.info(f"Battery Remaining: {msg.battery_remaining}%", extra={'system': 'STARTUP'})
        logger.info(f"Communication Drop Rate: {msg.drop_rate_comm:.1f}%", extra={'system': 'STARTUP'})
        logger.info(f"Error Count Comm: {msg.errors_comm}", extra={'system': 'STARTUP'})
        logger.info(f"Error Count 1: {msg.errors_count1}", extra={'system': 'STARTUP'})
        logger.info(f"Error Count 2: {msg.errors_count2}", extra={'system': 'STARTUP'})
        logger.info(f"Error Count 3: {msg.errors_count3}", extra={'system': 'STARTUP'})
        logger.info(f"Error Count 4: {msg.errors_count4}", extra={'system': 'STARTUP'})

    def _log_gps_status(self, msg):
        """Log GPS status information."""
        logger.info("--- GPS STATUS ---", extra={'system': 'STARTUP'})
        logger.info(f"GPS Fix Type: {msg.fix_type}", extra={'system': 'STARTUP'})
        logger.info(f"Satellites Visible: {msg.satellites_visible}", extra={'system': 'STARTUP'})
        logger.info(f"HDOP: {msg.eph / 100.0:.1f}", extra={'system': 'STARTUP'})
        logger.info(f"VDOP: {msg.epv / 100.0:.1f}", extra={'system': 'STARTUP'})
        logger.info(f"Velocity: {msg.vel / 100.0:.1f} cm/s", extra={'system': 'STARTUP'})
        logger.info(f"Course Over Ground: {msg.cog / 100.0:.1f} degrees", extra={'system': 'STARTUP'})
        
        # Convert GPS coordinates
        lat = msg.lat / 1e7
        lon = msg.lon / 1e7
        alt = msg.alt / 1000.0
        logger.info(f"Position: Lat {lat:.7f}, Lon {lon:.7f}, Alt {alt:.2f}m", extra={'system': 'STARTUP'})

    def _log_attitude(self, msg):
        """Log attitude information."""
        logger.info("--- ATTITUDE ---", extra={'system': 'STARTUP'})
        logger.info(f"Roll: {msg.roll:.3f} rad ({msg.roll * 180.0 / 3.14159:.1f}°)", extra={'system': 'STARTUP'})
        logger.info(f"Pitch: {msg.pitch:.3f} rad ({msg.pitch * 180.0 / 3.14159:.1f}°)", extra={'system': 'STARTUP'})
        logger.info(f"Yaw: {msg.yaw:.3f} rad ({msg.yaw * 180.0 / 3.14159:.1f}°)", extra={'system': 'STARTUP'})
        logger.info(f"Roll Speed: {msg.rollspeed:.3f} rad/s", extra={'system': 'STARTUP'})
        logger.info(f"Pitch Speed: {msg.pitchspeed:.3f} rad/s", extra={'system': 'STARTUP'})
        logger.info(f"Yaw Speed: {msg.yawspeed:.3f} rad/s", extra={'system': 'STARTUP'})

    def _log_global_position(self, msg):
        """Log global position information."""
        logger.info("--- GLOBAL POSITION ---", extra={'system': 'STARTUP'})
        logger.info(f"Latitude: {msg.lat / 1e7:.7f}°", extra={'system': 'STARTUP'})
        logger.info(f"Longitude: {msg.lon / 1e7:.7f}°", extra={'system': 'STARTUP'})
        logger.info(f"Altitude: {msg.alt / 1000.0:.2f}m", extra={'system': 'STARTUP'})
        logger.info(f"Relative Altitude: {msg.relative_alt / 1000.0:.2f}m", extra={'system': 'STARTUP'})
        logger.info(f"Velocity X: {msg.vx / 100.0:.2f} m/s", extra={'system': 'STARTUP'})
        logger.info(f"Velocity Y: {msg.vy / 100.0:.2f} m/s", extra={'system': 'STARTUP'})
        logger.info(f"Velocity Z: {msg.vz / 100.0:.2f} m/s", extra={'system': 'STARTUP'})
        logger.info(f"Heading: {msg.hdg / 100.0:.1f}°", extra={'system': 'STARTUP'})

    def get_rover_mode_name(self, mode_code):
        """Get ArduRover mode name from mode code."""
        return ROVER_MODES.get(mode_code, f"UNKNOWN_MODE_{mode_code}")

    def read_parameters(self, param_names=None, write_to_file=False):
        """Read MAVLink parameters from the rover."""
        if not self.master:
            logger.warning("Cannot read parameters: Not connected to rover", extra={'system': 'PARAMETERS'})
            return {}

        try:
            # Request parameter list
            self.master.mav.param_request_list_send(
                self.target_system,
                self.target_component
            )
            
            parameters = {}
            timeout_count = 0
            max_timeout = 10  # 10 seconds max
            last_param_count = 0
            stable_count = 0
            
            while timeout_count < max_timeout:
                msg = self.master.recv_match(type='PARAM_VALUE', blocking=True, timeout=1)
                if msg:
                    param_name = msg.param_id.rstrip('\x00') if isinstance(msg.param_id, str) else msg.param_id.decode('utf-8').rstrip('\x00')
                    param_value = msg.param_value
                    param_type = msg.param_type
                    
                    parameters[param_name] = {
                        'value': param_value,
                        'type': param_type
                    }
                    
                    # If we have specific parameters to read, check if we have them all
                    if param_names:
                        if all(name in parameters for name in param_names):
                            break
                    
                    # Check if parameter count is stable (no new parameters for 3 seconds)
                    if len(parameters) == last_param_count:
                        stable_count += 1
                        if stable_count >= 3:  # 3 seconds of no new parameters
                            logger.info(f"Parameter reading complete - no new parameters for 3 seconds", extra={'system': 'PARAMETERS'})
                            break
                    else:
                        stable_count = 0
                        last_param_count = len(parameters)
                    
                    timeout_count = 0  # Reset timeout counter
                else:
                    timeout_count += 1
            
            if parameters:
                logger.info(f"Read {len(parameters)} parameters", extra={'system': 'PARAMETERS'})
                
                # Write all parameters to file if requested
                if write_to_file:
                    self._write_parameters_to_file(parameters)
                
                # Log important rover parameters
                important_params = [
                    'RC_MAP_THROTTLE', 'RC_MAP_STEERING', 'RC_MAP_CHANNEL_SWITCH',
                    'RC_MAP_MODE_SWITCH', 'RC_MAP_ACRO_SWITCH', 'RC_MAP_SAFETY_SWITCH',
                    'RC_SPEED', 'RC_DEADBAND', 'RC_MIN_DWELL', 'RC_MAX_DWELL',
                    'RC_REVERSED_THROTTLE', 'RC_REVERSED_STEERING',
                    'WP_RADIUS', 'WP_SPEED', 'WP_SPEED_MAX', 'WP_SPEED_MIN',
                    'CRUISE_SPEED', 'CRUISE_THROTTLE', 'THR_MIN', 'THR_MAX',
                    'STEER2SRV_P', 'STEER2SRV_I', 'STEER2SRV_D',
                    'GPS_TYPE', 'GPS_AUTO_SWITCH', 'GPS_DELAY_MS',
                    'COMPASS_LEARN', 'COMPASS_USE', 'COMPASS_ORIENT',
                    'ARMING_CHECK', 'ARMING_RUDDER', 'ARMING_ACRO',
                    'BATT_MONITOR', 'BATT_CAPACITY', 'BATT_VOLT_PIN',
                    'SERVO1_FUNCTION', 'SERVO2_FUNCTION', 'SERVO3_FUNCTION',
                    'SERVO4_FUNCTION', 'SERVO5_FUNCTION', 'SERVO6_FUNCTION',
                    'SERVO7_FUNCTION', 'SERVO8_FUNCTION'
                ]
                
                logger.info("=== Important Rover Parameters ===", extra={'system': 'PARAMETERS'})
                
                for param_name in important_params:
                    if param_name in parameters:
                        param_info = parameters[param_name]
                        logger.info(f"{param_name}: {param_info['value']} (type: {param_info['type']})", 
                                   extra={'system': 'PARAMETERS'})
                
                # Log RC mapping parameters specifically
                rc_params = [name for name in parameters.keys() if name.startswith('RC_MAP_')]
                if rc_params:
                    logger.info("=== RC Channel Mapping ===", extra={'system': 'PARAMETERS'})
                    for param_name in sorted(rc_params):
                        param_info = parameters[param_name]
                        logger.info(f"{param_name}: {param_info['value']}", extra={'system': 'PARAMETERS'})
                
                # Log servo function parameters
                servo_params = [name for name in parameters.keys() if name.startswith('SERVO') and 'FUNCTION' in name]
                if servo_params:
                    logger.info("=== Servo Functions ===", extra={'system': 'PARAMETERS'})
                    for param_name in sorted(servo_params):
                        param_info = parameters[param_name]
                        logger.info(f"{param_name}: {param_info['value']}", extra={'system': 'PARAMETERS'})
                
                logger.info("=== End Parameters ===", extra={'system': 'PARAMETERS'})
                
                return parameters
            else:
                logger.warning("No parameters received", extra={'system': 'PARAMETERS'})
                return {}
                
        except Exception as e:
            logger.error(f"Error reading parameters: {e}", extra={'system': 'PARAMETERS'})
            return {}

    def _write_parameters_to_file(self, parameters):
        """Write all parameters to param.log file."""
        try:
            timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
            with open('param.log', 'w') as f:
                f.write(f"# Rover Parameters - {timestamp}\n")
                f.write(f"# Total parameters: {len(parameters)}\n")
                f.write("# Format: PARAM_NAME: VALUE (TYPE)\n\n")
                
                # Write all parameters sorted by name
                for param_name in sorted(parameters.keys()):
                    param_info = parameters[param_name]
                    f.write(f"{param_name}: {param_info['value']} ({param_info['type']})\n")
            
            logger.info(f"All {len(parameters)} parameters written to param.log", extra={'system': 'PARAMETERS'})
            
        except Exception as e:
            logger.error(f"Error writing parameters to file: {e}", extra={'system': 'PARAMETERS'})

    def read_specific_parameters(self, param_names):
        """Read specific parameters by name."""
        if not self.master:
            logger.warning("Cannot read specific parameters: Not connected to rover", extra={'system': 'PARAMETERS'})
            return {}
        
        logger.info(f"Reading specific parameters: {param_names}", extra={'system': 'PARAMETERS'})
        
        parameters = {}
        for param_name in param_names:
            try:
                # Request specific parameter
                self.master.mav.param_request_read_send(
                    self.target_system,
                    self.target_component,
                    param_name.encode('utf-8'),
                    -1  # Use -1 to request by name
                )
                
                # Wait for parameter value
                msg = self.master.recv_match(type='PARAM_VALUE', blocking=True, timeout=5)
                if msg:
                    received_param_name = msg.param_id.rstrip('\x00') if isinstance(msg.param_id, str) else msg.param_id.decode('utf-8').rstrip('\x00')
                    if received_param_name == param_name:
                        parameters[param_name] = {
                            'value': msg.param_value,
                            'type': msg.param_type
                        }
                        logger.info(f"{param_name}: {msg.param_value} (type: {msg.param_type})", 
                                   extra={'system': 'PARAMETERS'})
                    else:
                        logger.warning(f"Parameter {param_name} not found or timeout", extra={'system': 'PARAMETERS'})
                else:
                    logger.warning(f"Parameter {param_name} not found or timeout", extra={'system': 'PARAMETERS'})
                    
            except Exception as e:
                logger.error(f"Error reading parameter {param_name}: {e}", extra={'system': 'PARAMETERS'})
        
        return parameters

    def read_rc_parameters(self, write_to_file=False):
        """Read and display RC mapping and channel parameters."""
        if not self.master:
            logger.warning("Cannot read RC parameters: Not connected to rover", extra={'system': 'RC_PARAMETERS'})
            return {}
            
        logger.info("Reading RC parameters from rover...", extra={'system': 'RC_PARAMETERS'})
        
        try:
            # Request parameter list
            self.master.mav.param_request_list_send(
                self.target_system,
                self.target_component
            )
            
            parameters = {}
            timeout_count = 0
            max_timeout = 10  # 10 seconds max
            last_param_count = 0
            stable_count = 0
            
            while timeout_count < max_timeout:
                msg = self.master.recv_match(type='PARAM_VALUE', blocking=True, timeout=1)
                if msg:
                    param_name = msg.param_id.rstrip('\x00') if isinstance(msg.param_id, str) else msg.param_id.decode('utf-8').rstrip('\x00')
                    param_value = msg.param_value
                    param_type = msg.param_type
                    
                    parameters[param_name] = {
                        'value': param_value,
                        'type': param_type
                    }
                    
                    # Check if parameter count is stable (no new parameters for 3 seconds)
                    if len(parameters) == last_param_count:
                        stable_count += 1
                        if stable_count >= 3:  # 3 seconds of no new parameters
                            logger.info(f"Parameter reading complete - no new parameters for 3 seconds", extra={'system': 'RC_PARAMETERS'})
                            break
                    else:
                        stable_count = 0
                        last_param_count = len(parameters)
                    
                    timeout_count = 0  # Reset timeout counter
                else:
                    timeout_count += 1
            
            if parameters:
                logger.info(f"Read {len(parameters)} parameters, filtering for RC parameters", extra={'system': 'RC_PARAMETERS'})
                
                # Write all parameters to file if requested
                if write_to_file:
                    self._write_parameters_to_file(parameters)
                
                # Filter for RC-related parameters
                rc_mapping_params = []
                rc_channel_params = []
                rc_other_params = []
                
                for param_name, param_info in parameters.items():
                    if param_name.startswith('RCMAP_'):
                        rc_mapping_params.append((param_name, param_info))
                    elif param_name.startswith('RC') and any(x in param_name for x in ['_MIN', '_TRIM', '_MAX', '_REVERSED', '_SPEED', '_DEADBAND']):
                        rc_channel_params.append((param_name, param_info))
                    elif param_name.startswith('RC') and not param_name.startswith('RC_'):
                        rc_other_params.append((param_name, param_info))
                
                # Display RC mapping parameters
                if rc_mapping_params:
                    logger.info("=== RC Channel Mapping ===", extra={'system': 'RC_PARAMETERS'})
                    for param_name, param_info in sorted(rc_mapping_params):
                        logger.info(f"{param_name}: {param_info['value']} (type: {param_info['type']})", 
                                   extra={'system': 'RC_PARAMETERS'})
                else:
                    logger.warning("No RC mapping parameters found", extra={'system': 'RC_PARAMETERS'})
                
                # Display RC channel parameters (MIN/TRIM/MAX)
                if rc_channel_params:
                    logger.info("=== RC Channel Settings ===", extra={'system': 'RC_PARAMETERS'})
                    # Group by channel number
                    channels = {}
                    for param_name, param_info in rc_channel_params:
                        # Extract channel number (e.g., RC1_MIN -> 1)
                        if param_name.startswith('RC') and param_name[2].isdigit():
                            channel_num = param_name[2]
                            if channel_num not in channels:
                                channels[channel_num] = []
                            channels[channel_num].append((param_name, param_info))
                    
                    for channel_num in sorted(channels.keys()):
                        logger.info(f"--- Channel {channel_num} ---", extra={'system': 'RC_PARAMETERS'})
                        for param_name, param_info in sorted(channels[channel_num]):
                            logger.info(f"  {param_name}: {param_info['value']}", extra={'system': 'RC_PARAMETERS'})
                
                # Display other RC parameters
                if rc_other_params:
                    logger.info("=== Other RC Parameters ===", extra={'system': 'RC_PARAMETERS'})
                    for param_name, param_info in sorted(rc_other_params):
                        logger.info(f"{param_name}: {param_info['value']} (type: {param_info['type']})", 
                                   extra={'system': 'RC_PARAMETERS'})
                
                logger.info("=== End RC Parameters ===", extra={'system': 'RC_PARAMETERS'})
                
                return parameters
            else:
                logger.warning("No parameters received", extra={'system': 'RC_PARAMETERS'})
                return {}
                
        except Exception as e:
            logger.error(f"Error reading RC parameters: {e}", extra={'system': 'RC_PARAMETERS'})
            return {}

    def _check_command_result(self, msg, operation_name):
        """Check command acknowledgment and log result."""
        if msg:
            if msg.result == mavutil.mavlink.MAV_RESULT_ACCEPTED:
                logger.info(f"{operation_name} command ACCEPTED - command executed successfully", extra={'system': 'COMMAND'})
                return True
            else:
                # Get human-readable result name
                result_name = "UNKNOWN"
                result_description = ""
                try:
                    if msg.result in mavutil.mavlink.enums.get('MAV_RESULT', {}):
                        result_name = mavutil.mavlink.enums['MAV_RESULT'][msg.result].name
                        # Add descriptions for common results
                        result_descriptions = {
                            'MAV_RESULT_TEMPORARILY_REJECTED': 'Command temporarily rejected - may succeed if retried later',
                            'MAV_RESULT_DENIED': 'Command denied - invalid parameters or conditions not met',
                            'MAV_RESULT_UNSUPPORTED': 'Command not supported or unknown',
                            'MAV_RESULT_FAILED': 'Command execution failed - check system status and pre-arm checks',
                            'MAV_RESULT_IN_PROGRESS': 'Command is in progress - waiting for completion',
                            'MAV_RESULT_CANCELLED': 'Command was cancelled'
                        }
                        result_description = result_descriptions.get(result_name, '')
                except (KeyError, AttributeError):
                    result_name = f"RESULT_{msg.result}"
                
                logger.error(f"{operation_name} command FAILED: {result_name} (code: {msg.result})", extra={'system': 'COMMAND'})
                if result_description:
                    logger.error(f"  Reason: {result_description}", extra={'system': 'COMMAND'})
                
                # Log additional parameters if present
                if hasattr(msg, 'result_param1') and msg.result_param1 != 0:
                    logger.error(f"  Additional info (param1): {msg.result_param1}", extra={'system': 'COMMAND'})
                if hasattr(msg, 'result_param2') and msg.result_param2 != 0:
                    logger.error(f"  Additional info (param2): {msg.result_param2}", extra={'system': 'COMMAND'})
                
                return False
        else:
            logger.error(f"{operation_name} command TIMED OUT - no acknowledgment received within timeout period", extra={'system': 'COMMAND'})
            return False

    def _start_error_monitoring(self):
        """Start monitoring for error messages in a non-blocking way."""
        # Clear any existing messages in the buffer
        self.master.recv_match(blocking=False)

    def _monitor_for_errors_duration(self, duration):
        """Monitor for error messages for a specified duration."""
        start_time = time.time()
        error_count = 0
        warning_count = 0
        
        while time.time() - start_time < duration:
            try:
                # Check for any incoming message
                msg = self.master.recv_match(blocking=False)
                if msg:
                    msg_type = msg.get_type()
                    
                    # Monitor for error messages
                    if msg_type == 'STATUSTEXT':
                        severity = msg.severity
                        text = msg.text.strip('\x00')
                        
                        if severity == mavutil.mavlink.MAV_SEVERITY_EMERGENCY:
                            logger.error(f"EMERGENCY: {text}", extra={'system': 'ARMING'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_ALERT:
                            logger.error(f"ALERT: {text}", extra={'system': 'ARMING'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_CRITICAL:
                            logger.error(f"CRITICAL: {text}", extra={'system': 'ARMING'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_ERROR:
                            logger.error(f"ERROR: {text}", extra={'system': 'ARMING'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_WARNING:
                            logger.warning(f"WARNING: {text}", extra={'system': 'ARMING'})
                            warning_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_NOTICE:
                            logger.info(f"NOTICE: {text}", extra={'system': 'ARMING'})
                    
                    # Monitor system status
                    elif msg_type == 'SYS_STATUS':
                        if msg.onboard_control_sensors_health != 0xFFFFFFFF:
                            logger.warning(f"System health issue detected: {msg.onboard_control_sensors_health:08X}", 
                                         extra={'system': 'ARMING'})
                            warning_count += 1
                    
                    # Monitor GPS status
                    elif msg_type == 'GPS_RAW_INT':
                        if msg.fix_type < 3:
                            logger.warning(f"GPS fix lost: Type {msg.fix_type}, Satellites: {msg.satellites_visible}", 
                                         extra={'system': 'ARMING'})
                            warning_count += 1
                
                time.sleep(0.1)
                
            except Exception as e:
                logger.error(f"Error during monitoring: {e}", extra={'system': 'ARMING'})
                time.sleep(1)
        
        if error_count > 0 or warning_count > 0:
            logger.info(f"Error monitoring complete - Errors: {error_count}, Warnings: {warning_count}", 
                       extra={'system': 'ARMING'})

    def _log_arm_failure_details(self, ack_msg):
        """Log detailed information about arm failure."""
        logger.error("=" * 60, extra={'system': 'ARMING'})
        logger.error("ARMING FAILURE DETAILS", extra={'system': 'ARMING'})
        logger.error("=" * 60, extra={'system': 'ARMING'})
        
        # Log the result code and name
        logger.error(f"MAV_RESULT code: {ack_msg.result}", extra={'system': 'ARMING'})
        try:
            if ack_msg.result in mavutil.mavlink.enums.get('MAV_RESULT', {}):
                result_name = mavutil.mavlink.enums['MAV_RESULT'][ack_msg.result].name
                logger.error(f"Result: {result_name}", extra={'system': 'ARMING'})
                
                # Provide specific guidance based on result type
                if result_name == 'MAV_RESULT_TEMPORARILY_REJECTED':
                    logger.error("  -> Arming temporarily rejected. Common reasons:", extra={'system': 'ARMING'})
                    logger.error("     - Waiting for GPS lock", extra={'system': 'ARMING'})
                    logger.error("     - Pre-arm checks in progress", extra={'system': 'ARMING'})
                    logger.error("     - System initialization not complete", extra={'system': 'ARMING'})
                elif result_name == 'MAV_RESULT_DENIED':
                    logger.error("  -> Arming denied. Check:", extra={'system': 'ARMING'})
                    logger.error("     - Pre-arm checks (compass, GPS, etc.)", extra={'system': 'ARMING'})
                    logger.error("     - Safety switch position", extra={'system': 'ARMING'})
                    logger.error("     - Arming checks enabled in parameters", extra={'system': 'ARMING'})
                elif result_name == 'MAV_RESULT_FAILED':
                    logger.error("  -> Arming failed. Check:", extra={'system': 'ARMING'})
                    logger.error("     - Sensor calibration status", extra={'system': 'ARMING'})
                    logger.error("     - System health status", extra={'system': 'ARMING'})
                    logger.error("     - Error messages below", extra={'system': 'ARMING'})
        except (KeyError, AttributeError):
            logger.error(f"Unknown result code: {ack_msg.result}", extra={'system': 'ARMING'})
        
        # Log additional parameters
        if hasattr(ack_msg, 'result_param1') and ack_msg.result_param1 != 0:
            logger.error(f"Additional parameter 1: {ack_msg.result_param1}", extra={'system': 'ARMING'})
        if hasattr(ack_msg, 'result_param2') and ack_msg.result_param2 != 0:
            logger.error(f"Additional parameter 2: {ack_msg.result_param2}", extra={'system': 'ARMING'})
        
        # Collect status text messages that might explain the failure
        logger.info("Collecting status messages that may explain the failure...", extra={'system': 'ARMING'})
        
        # Check for any pending status text messages first (they might already be in buffer)
        status_messages = []
        for _ in range(10):  # Check for up to 10 status messages
            status_msg = self.master.recv_match(type='STATUSTEXT', blocking=False)
            if status_msg:
                text = status_msg.text.strip('\x00')
                severity = status_msg.severity
                status_messages.append((severity, text))
            else:
                break
        
        # Also wait a bit for new status messages
        start_time = time.time()
        while time.time() - start_time < 2:
            status_msg = self.master.recv_match(type='STATUSTEXT', blocking=False)
            if status_msg:
                text = status_msg.text.strip('\x00')
                severity = status_msg.severity
                if (severity, text) not in status_messages:
                    status_messages.append((severity, text))
            time.sleep(0.1)
        
        # Log collected status messages
        if status_messages:
            logger.error("Status messages received:", extra={'system': 'ARMING'})
            for severity, text in status_messages:
                severity_names = {
                    mavutil.mavlink.MAV_SEVERITY_EMERGENCY: 'EMERGENCY',
                    mavutil.mavlink.MAV_SEVERITY_ALERT: 'ALERT',
                    mavutil.mavlink.MAV_SEVERITY_CRITICAL: 'CRITICAL',
                    mavutil.mavlink.MAV_SEVERITY_ERROR: 'ERROR',
                    mavutil.mavlink.MAV_SEVERITY_WARNING: 'WARNING',
                    mavutil.mavlink.MAV_SEVERITY_NOTICE: 'NOTICE',
                    mavutil.mavlink.MAV_SEVERITY_INFO: 'INFO',
                    mavutil.mavlink.MAV_SEVERITY_DEBUG: 'DEBUG'
                }
                severity_name = severity_names.get(severity, f'SEVERITY_{severity}')
                if severity <= mavutil.mavlink.MAV_SEVERITY_ERROR:
                    logger.error(f"  [{severity_name}] {text}", extra={'system': 'ARMING'})
                elif severity == mavutil.mavlink.MAV_SEVERITY_WARNING:
                    logger.warning(f"  [{severity_name}] {text}", extra={'system': 'ARMING'})
                else:
                    logger.info(f"  [{severity_name}] {text}", extra={'system': 'ARMING'})
        else:
            logger.warning("No status messages received that explain the failure", extra={'system': 'ARMING'})
        
        logger.error("=" * 60, extra={'system': 'ARMING'})

    def get_status(self, message):
        """Gets the status of the rover and logs it to the console."""
        print(message)
        self.master.mav.request_data_stream_send(self.master.target_system, self.master.target_component, mavutil.mavlink.MAV_DATA_STREAM_ALL, 1, 1)
        msg = self.master.recv_match(type='SYS_STATUS', blocking=True)
        if msg:
            print("System status: %s" % msg)

    def arm(self):
        """Arms the rover."""
        logger.info("Attempting to arm rover", extra={'system': 'ARMING'})
        
        # Log current status before arming
        self._log_heartbeat()
        
        # Check if already armed
        if self.last_heartbeat and bool(self.last_heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED):
            logger.warning("Rover is already armed", extra={'system': 'ARMING'})
            return True
        
        # Start monitoring for error messages before sending arm command
        logger.info("Starting error monitoring before arming attempt", extra={'system': 'ARMING'})
        self._start_error_monitoring()
        
        # Send arm command
        self.master.mav.command_long_send(
            self.target_system,
            self.target_component,
            mavutil.mavlink.MAV_CMD_COMPONENT_ARM_DISARM,
            0,
            1, 0, 0, 0, 0, 0, 0)
        
        # Wait for command acknowledgment
        msg = self.master.recv_match(type='COMMAND_ACK', blocking=True, timeout=5)
        success = self._check_command_result(msg, "Arming")
        
        # If arming failed, try to get detailed error information
        if not success and msg:
            self._log_arm_failure_details(msg)
        
        # Continue monitoring for a few seconds to catch any delayed error messages
        logger.info("Monitoring for delayed error messages after arming attempt", extra={'system': 'ARMING'})
        self._monitor_for_errors_duration(3)
        
        if success:
            # Wait a moment and check if actually armed
            logger.info("Waiting for arming to complete...", extra={'system': 'ARMING'})
            time.sleep(1)
            self._log_heartbeat()
            
            if self.last_heartbeat and bool(self.last_heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED):
                logger.info("=" * 60, extra={'system': 'ARMING'})
                logger.info("✓ ARMING SUCCESSFUL", extra={'system': 'ARMING'})
                logger.info("=" * 60, extra={'system': 'ARMING'})
                logger.info("Rover is now ARMED and ready for operation", extra={'system': 'ARMING'})
                mode_name = self.get_rover_mode_name(self.last_heartbeat.custom_mode)
                logger.info(f"Current mode: {mode_name}", extra={'system': 'ARMING'})
                return True
            else:
                logger.error("=" * 60, extra={'system': 'ARMING'})
                logger.error("ARMING COMMAND ACCEPTED BUT ROVER NOT ARMED", extra={'system': 'ARMING'})
                logger.error("=" * 60, extra={'system': 'ARMING'})
                logger.error("The arming command was accepted but the rover did not arm.", extra={'system': 'ARMING'})
                logger.error("This may indicate a safety check or system issue.", extra={'system': 'ARMING'})
                return False
        
        return False

    def disarm(self):
        """Disarms the rover."""
        logger.info("Attempting to disarm rover", extra={'system': 'DISARMING'})
        
        # Log current status before disarming
        self._log_heartbeat()
        
        # Check if already disarmed
        if self.last_heartbeat and not bool(self.last_heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED):
            logger.warning("Rover is already disarmed", extra={'system': 'DISARMING'})
            return True
        
        # Send disarm command
        self.master.mav.command_long_send(
            self.target_system,
            self.target_component,
            mavutil.mavlink.MAV_CMD_COMPONENT_ARM_DISARM,
            0,
            0, 0, 0, 0, 0, 0, 0)
        
        # Wait for command acknowledgment
        msg = self.master.recv_match(type='COMMAND_ACK', blocking=True, timeout=5)
        success = self._check_command_result(msg, "Disarming")
        
        if success:
            # Wait a moment and check if actually disarmed
            time.sleep(1)
            self._log_heartbeat()
            
            if self.last_heartbeat and not bool(self.last_heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED):
                logger.info("Rover successfully disarmed", extra={'system': 'DISARMING'})
                return True
            else:
                logger.error("Disarming command accepted but rover still armed", extra={'system': 'DISARMING'})
                return False
        
        return False

    def check_armed(self, message):
        """Checks if the rover is armed and logs it to the console."""
        logger.info(message, extra={'system': 'STATUS_CHECK'})
        self._log_heartbeat()
        
        if self.last_heartbeat:
            if self.last_heartbeat.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED:
                logger.info("Rover is armed", extra={'system': 'STATUS_CHECK'})
                return True
        else:
                logger.info("Rover is not armed", extra={'system': 'STATUS_CHECK'})
                return False
        return None

    def get_current_mode(self):
        """Gets the current flight mode of the rover."""
        self._log_heartbeat()
        if self.last_heartbeat:
            custom_mode = self.last_heartbeat.custom_mode
            base_mode = self.last_heartbeat.base_mode
            mode_name = self.get_rover_mode_name(custom_mode)
            logger.info(f"Current mode: {mode_name} ({custom_mode}), Base mode: {base_mode}", 
                       extra={'system': 'MODE_CHECK'})
            return custom_mode, base_mode
        return None, None

    def set_guided_mode(self):
        """Sets the rover to guided mode."""
        logger.info("Setting rover to GUIDED mode", extra={'system': 'MODE_CHANGE'})
        
        # Get current mode before change
        current_mode, base_mode = self.get_current_mode()
        if current_mode is not None:
            current_mode_name = self.get_rover_mode_name(current_mode)
            logger.info(f"Current mode: {current_mode_name} ({current_mode})", extra={'system': 'MODE_CHANGE'})
            
            # Check if already in guided mode
            if current_mode == 15:  # GUIDED mode
                logger.info("Rover is already in GUIDED mode", extra={'system': 'MODE_CHANGE'})
                return True
        
        # Send guided mode command (mode 15 for ArduRover)
        self.master.mav.command_long_send(
            self.target_system,
            self.target_component,
            mavutil.mavlink.MAV_CMD_DO_SET_MODE,
            0,
            mavutil.mavlink.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED,
            15,  # GUIDED mode for ArduRover
            0, 0, 0, 0, 0)
        
        # Wait for command acknowledgment
        msg = self.master.recv_match(type='COMMAND_ACK', blocking=True, timeout=5)
        success = self._check_command_result(msg, "Guided mode")
        
        if success:
            # Wait for mode change
            if self.wait_for_mode_change(15, timeout=10):
                logger.info("Successfully set to GUIDED mode", extra={'system': 'MODE_CHANGE'})
                return True
            else:
                logger.error("Mode change to GUIDED timed out", extra={'system': 'MODE_CHANGE'})
                return False
        
            return False

    def wait_for_mode_change(self, target_mode, timeout=10):
        """Waits for mode change to complete."""
        target_mode_name = self.get_rover_mode_name(target_mode)
        logger.info(f"Waiting for mode change to {target_mode_name} ({target_mode})", extra={'system': 'MODE_CHANGE'})
        start_time = time.time()
        
        while time.time() - start_time < timeout:
            current_mode, _ = self.get_current_mode()
            if current_mode == target_mode:
                logger.info(f"Mode successfully changed to {target_mode_name} ({target_mode})", extra={'system': 'MODE_CHANGE'})
                return True
            time.sleep(0.5)
        
        logger.error(f"Timeout waiting for mode change to {target_mode_name} ({target_mode})", extra={'system': 'MODE_CHANGE'})
        return False

    def set_mode(self, mode_number):
        """Generic function to set rover to any mode."""
        mode_name = self.get_rover_mode_name(mode_number)
        logger.info(f"Setting rover to {mode_name} mode (mode {mode_number})", extra={'system': 'MODE_CHANGE'})
        
        # Get current mode before change
        current_mode, base_mode = self.get_current_mode()
        if current_mode is not None:
            current_mode_name = self.get_rover_mode_name(current_mode)
            logger.info(f"Current mode: {current_mode_name} ({current_mode})", extra={'system': 'MODE_CHANGE'})
            
            # Check if already in target mode
            if current_mode == mode_number:
                logger.info(f"Rover is already in {mode_name} mode", extra={'system': 'MODE_CHANGE'})
                return True
        
        # Send mode change command
        self.master.mav.command_long_send(
            self.target_system,
            self.target_component,
            mavutil.mavlink.MAV_CMD_DO_SET_MODE,
            0,
            mavutil.mavlink.MAV_MODE_FLAG_CUSTOM_MODE_ENABLED,
            mode_number,  # Custom mode
            0, 0, 0, 0, 0)
        
        # Wait for command acknowledgment
        msg = self.master.recv_match(type='COMMAND_ACK', blocking=True, timeout=5)
        success = self._check_command_result(msg, f"{mode_name} mode")
        
        if success:
            # Wait for mode change
            if self.wait_for_mode_change(mode_number, timeout=10):
                logger.info(f"Successfully set to {mode_name} mode", extra={'system': 'MODE_CHANGE'})
                return True
            else:
                logger.error(f"Mode change to {mode_name} timed out", extra={'system': 'MODE_CHANGE'})
                return False
        
            return False

    def set_manual_mode(self):
        """Sets the rover to manual mode."""
        return self.set_mode(0)  # Mode 0 = MANUAL in ArduRover

    def monitor_errors(self, duration=10):
        """Monitor for errors and status messages for a specified duration."""
        logger.info(f"Starting error monitoring for {duration} seconds", extra={'system': 'ERROR_MONITOR'})
        
        start_time = time.time()
        error_count = 0
        warning_count = 0
        
        while time.time() - start_time < duration:
            try:
                # Check for any incoming message
                msg = self.master.recv_match(blocking=False)
                if msg:
                    msg_type = msg.get_type()
                    
                    # Monitor for error messages
                    if msg_type == 'STATUSTEXT':
                        severity = msg.severity
                        text = msg.text.strip('\x00')
                        
                        if severity == mavutil.mavlink.MAV_SEVERITY_EMERGENCY:
                            logger.error(f"EMERGENCY: {text}", extra={'system': 'ERROR_MONITOR'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_ALERT:
                            logger.error(f"ALERT: {text}", extra={'system': 'ERROR_MONITOR'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_CRITICAL:
                            logger.error(f"CRITICAL: {text}", extra={'system': 'ERROR_MONITOR'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_ERROR:
                            logger.error(f"ERROR: {text}", extra={'system': 'ERROR_MONITOR'})
                            error_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_WARNING:
                            logger.warning(f"WARNING: {text}", extra={'system': 'ERROR_MONITOR'})
                            warning_count += 1
                        elif severity == mavutil.mavlink.MAV_SEVERITY_NOTICE:
                            logger.info(f"NOTICE: {text}", extra={'system': 'ERROR_MONITOR'})
                    
                    # Monitor system status
                    elif msg_type == 'SYS_STATUS':
                        if msg.onboard_control_sensors_health != 0xFFFFFFFF:
                            logger.warning(f"System health issue detected: {msg.onboard_control_sensors_health:08X}", 
                                         extra={'system': 'ERROR_MONITOR'})
                            warning_count += 1
                    
                    # Monitor GPS status
                    elif msg_type == 'GPS_RAW_INT':
                        if msg.fix_type < 3:
                            logger.warning(f"GPS fix lost: Type {msg.fix_type}, Satellites: {msg.satellites_visible}", 
                                         extra={'system': 'ERROR_MONITOR'})
                            warning_count += 1
                
                time.sleep(0.1)
                
            except Exception as e:
                logger.error(f"Error during monitoring: {e}", extra={'system': 'ERROR_MONITOR'})
                time.sleep(1)
        
        logger.info(f"Error monitoring complete - Errors: {error_count}, Warnings: {warning_count}", 
                   extra={'system': 'ERROR_MONITOR'})
        return error_count, warning_count

    def verify_mode_change(self, target_mode, timeout=10):
        """Verifies that mode change was successful."""
        target_mode_name = self.get_rover_mode_name(target_mode)
        logger.info(f"Verifying mode change to {target_mode_name} ({target_mode})", extra={'system': 'MODE_VERIFY'})
        
        # Wait for mode change
        if self.wait_for_mode_change(target_mode, timeout):
            # Double-check the mode
            final_mode, _ = self.get_current_mode()
            if final_mode == target_mode:
                logger.info(f"Mode change verification successful: {target_mode_name} ({target_mode})", extra={'system': 'MODE_VERIFY'})
                return True
            else:
                final_mode_name = self.get_rover_mode_name(final_mode)
                logger.error(f"Mode change verification failed: expected {target_mode_name} ({target_mode}), got {final_mode_name} ({final_mode})", extra={'system': 'MODE_VERIFY'})
                return False
        else:
            logger.error(f"Mode change verification failed: timeout waiting for mode {target_mode_name} ({target_mode})", extra={'system': 'MODE_VERIFY'})
            return False

    def monitor(self):
        """Continuously monitors the rover and logs heartbeat messages."""
        logger.info("Starting rover monitoring (heartbeat only)", extra={'system': 'MONITOR'})
        logger.info("Press Ctrl+C to stop monitoring", extra={'system': 'MONITOR'})
        
        try:
            while True:
                # Listen for any incoming message
                msg = self.master.recv_match(blocking=True, timeout=1.0)
                
                if msg is not None:
                    # Log only heartbeat messages
                    if msg.get_type() == 'HEARTBEAT':
                        self._log_heartbeat()
                
        except KeyboardInterrupt:
            logger.info("Monitoring stopped by user", extra={'system': 'MONITOR'})
        except Exception as e:
            logger.error(f"Monitoring error: {e}", extra={'system': 'MONITOR'})

    def get_gps_position(self):
        """Get current GPS position from the rover."""
        if not self.master:
            logger.warning("Cannot get GPS position: Not connected to rover", extra={'system': 'GPS'})
            return False

        logger.info("Requesting GPS position...", extra={'system': 'GPS'})
        
        try:
            # Request GPS raw data
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_GPS_RAW_INT,
                0, 0, 0, 0, 0, 0)
            
            # Request global position
            self.master.mav.command_long_send(
                self.target_system,
                self.target_component,
                mavutil.mavlink.MAV_CMD_REQUEST_MESSAGE,
                0,
                mavutil.mavlink.MAVLINK_MSG_ID_GLOBAL_POSITION_INT,
                0, 0, 0, 0, 0, 0)
            
            # Wait for GPS messages
            gps_raw_received = False
            global_position_received = False
            start_time = time.time()
            
            while time.time() - start_time < 5:
                msg = self.master.recv_match(blocking=False)
                if msg:
                    msg_type = msg.get_type()
                    
                    if msg_type == 'GPS_RAW_INT' and not gps_raw_received:
                        self._log_gps_status(msg)
                        gps_raw_received = True
                    
                    elif msg_type == 'GLOBAL_POSITION_INT' and not global_position_received:
                        self._log_global_position(msg)
                        global_position_received = True
                    
                    if gps_raw_received and global_position_received:
                        break
                
                time.sleep(0.1)
            
            if not gps_raw_received:
                logger.warning("GPS_RAW_INT not received", extra={'system': 'GPS'})
            if not global_position_received:
                logger.warning("GLOBAL_POSITION_INT not received", extra={'system': 'GPS'})
            
            return gps_raw_received or global_position_received
            
        except Exception as e:
            logger.error(f"Error getting GPS position: {e}", extra={'system': 'GPS'})
            return False

    def serve(self):
        """
        Persistent MAVLink connection handler.
        Streams telemetry to stdout as JSON and reads commands from stdin.
        """
        logger.info("Starting persistent MAVLink server...", extra={'system': 'SERVE'})
        
        # Set message rates (Hz)
        try:
            # Attitude (10Hz)
            self.master.mav.command_long_send(
                self.target_system, self.target_component,
                mavutil.mavlink.MAV_CMD_SET_MESSAGE_INTERVAL, 0,
                mavutil.mavlink.MAVLINK_MSG_ID_ATTITUDE, 100000, 
                0, 0, 0, 0, 0)
            
            # VFR_HUD (5Hz)
            self.master.mav.command_long_send(
                self.target_system, self.target_component,
                mavutil.mavlink.MAV_CMD_SET_MESSAGE_INTERVAL, 0,
                mavutil.mavlink.MAVLINK_MSG_ID_VFR_HUD, 200000,
                0, 0, 0, 0, 0)
                
            # Heartbeat (1Hz)
            self.master.mav.command_long_send(
                self.target_system, self.target_component,
                mavutil.mavlink.MAV_CMD_SET_MESSAGE_INTERVAL, 0,
                mavutil.mavlink.MAVLINK_MSG_ID_HEARTBEAT, 1000000,
                0, 0, 0, 0, 0)
        except Exception as e:
            logger.error(f"Error setting message intervals: {e}", extra={'system': 'SERVE'})

        last_telemetry = 0
        
        try:
            while True:
                # Check for messages from MAVLink
                msg = self.master.recv_match(blocking=False)
                if msg:
                    self._handle_mavlink_message(msg)
                    
                # Check for commands from stdin
                if select.select([sys.stdin], [], [], 0)[0]:
                    line = sys.stdin.readline()
                    if line:
                        self._handle_stdin_command(line.strip())
                
                # Small sleep to prevent 100% CPU
                time.sleep(0.01)
                
        except KeyboardInterrupt:
            logger.info("Stopping persistent server...")
        except Exception as e:
            logger.error(f"Server error: {e}")
                
    def _handle_mavlink_message(self, msg):
        """Handle incoming MAVLink messages and update state."""
        msg_type = msg.get_type()
        now = time.time()
        self.state.last_update = now

        # Always log important messages
        if msg_type in ['HEARTBEAT', 'STATUSTEXT', 'COMMAND_ACK', 'SYS_STATUS']:
             if msg_type == 'HEARTBEAT':
                 self._log_heartbeat()
             elif msg_type == 'STATUSTEXT':
                 severity = msg.severity
                 text = msg.text.strip('\x00')
                 logger.info(f"STATUS: {text} (severity: {severity})")
             elif msg_type == 'COMMAND_ACK':
                 self._check_command_result(msg, "Remote Command")

        # Update state based on message type
        if msg_type == 'ATTITUDE':
            self.state.roll = msg.roll
            self.state.pitch = msg.pitch
            self.state.yaw = msg.yaw
        elif msg_type == 'VFR_HUD':
            self.state.speed = msg.groundspeed
            self.state.heading = msg.heading
        elif msg_type == 'HEARTBEAT':
            self.state.armed = bool(msg.base_mode & mavutil.mavlink.MAV_MODE_FLAG_SAFETY_ARMED)
            self.state.mode = ROVER_MODES.get(msg.custom_mode, f"MODE_{msg.custom_mode}")
            self.last_heartbeat = msg
        elif msg_type == 'SYS_STATUS':
            self.state.battery_voltage = msg.voltage_battery / 1000.0
            self.state.battery_remaining = msg.battery_remaining
            # Update EKF health (simplified)
            self.state.ekf.healthy = (msg.onboard_control_sensors_health & (1 << 21)) != 0 # EKF status bit
            self.state.ekf.status = "HEALTHY" if self.state.ekf.healthy else "UNHEALTHY"
        elif msg_type == 'GPS_RAW_INT':
            self.state.gps.latitude = msg.lat / 1e7
            self.state.gps.longitude = msg.lon / 1e7
            self.state.gps.altitude = msg.alt / 1000.0
            self.state.gps.fix_type = msg.fix_type
            self.state.gps.satellites = msg.satellites_visible

        # Update the legacy telemetry dict for backward compatibility
        self.telemetry = self.state.to_dict()

        # Emit consolidated telemetry at ~10Hz if serve() is running
        now = time.time()
        if hasattr(self, '_last_telemetry_emit') and now - self._last_telemetry_emit >= 0.1:
            self._last_telemetry_emit = now
            print(json.dumps(self.telemetry))
            sys.stdout.flush()

    def _handle_stdin_command(self, command):
        """Process commands received from stdin."""
        if not command:
            return
            
        logger.info(f"Received command via stdin: {command}", extra={'system': 'SERVE'})
        
        try:
            if command == "--arm":
                self.arm()
            elif command == "--disarm":
                self.disarm()
            elif command == "--status":
                self.check_armed("Status Check")
                self.get_current_mode()
            elif command == "--gps":
                self.get_gps_position()
            elif command.startswith("--mode "):
                mode_name = command.split(" ", 1)[1].upper()
                mode_code = None
                for code, name in ROVER_MODES.items():
                    if name == mode_name:
                        mode_code = code
                        break
                if mode_code is not None:
                    self.set_mode(mode_code)
                else:
                    logger.error(f"Unknown mode: {mode_name}", extra={'system': 'SERVE'})
            else:
                logger.warning(f"Unsupported command: {command}", extra={'system': 'SERVE'})
        except Exception as e:
            logger.error(f"Error executing command '{command}': {e}", extra={'system': 'SERVE'})

    async def publish_state(self, nc):
        """Publish the current rover state to NATS."""
        state_dict = self.state.to_dict()
        try:
            await nc.publish("rover.state", json.dumps(state_dict).encode())
        except Exception as e:
            logger.error(f"Error publishing to NATS: {e}")

async def telemetry_publish_loop(rover: Rover, nc, nats_url="nats://localhost:4222"):
    """Periodically publish telemetry to NATS."""
    logger.info(f"Starting NATS publish loop...", extra={'system': 'NATS'})
    
    while True:
        try:
            if rover.mock_mode:
                import math
                elapsed = time.time() - rover.mock_start_time
                # Generate sliding mock data
                rover.state.speed = 1.0 + math.sin(elapsed * 0.5) * 0.5
                rover.state.heading = int((elapsed * 10) % 360)
                rover.state.battery_voltage = 12.0 + math.sin(elapsed * 0.1) * 0.5
                rover.state.battery_remaining = int(80 + math.sin(elapsed * 0.1) * 10)
                rover.state.gps.latitude = 37.7749 + math.sin(elapsed * 0.05) * 0.001
                rover.state.gps.longitude = -122.4194 + math.cos(elapsed * 0.05) * 0.001
                rover.state.gps.altitude = 10.0 + math.sin(elapsed * 0.2) * 2
                rover.state.gps.satellites = 12
                rover.state.gps.fix_type = 3
                rover.state.ekf.healthy = True
                rover.state.ekf.status = "MOCKING"
                rover.state.mode = "MOCK"
                rover.state.armed = True
            
            await rover.publish_state(nc)
            await asyncio.sleep(0.1) # 10Hz
            
        except Exception as e:
            logger.error(f"Error in NATS publish loop: {e}")
            await asyncio.sleep(1)

async def command_subscribe_loop(rover: Rover, nc):
    """Subscribe to commands from NATS."""
    logger.info("Starting NATS command subscription loop...", extra={'system': 'NATS'})
    
    async def message_handler(msg):
        subject = msg.subject
        data = msg.data.decode()
        logger.info(f"Received NATS command on {subject}: {data}")
        try:
            command = json.loads(data)
            await handle_command(command, rover)
        except json.JSONDecodeError:
            # Try raw command
            rover._handle_stdin_command(data)

    await nc.subscribe("rover.command", cb=message_handler)
    
    # Keep the task alive
    while True:
        await asyncio.sleep(1)

async def handle_command(command: Dict[str, Any], rover: Rover):
    """Handle commands received via NATS."""
    cmd_type = command.get("type")
    
    if cmd_type == "connect":
        rover.connect()
    elif cmd_type == "arm":
        rover.arm()
    elif cmd_type == "disarm":
        rover.disarm()
    elif cmd_type == "set_mode":
        mode_name = command.get("mode", "").upper()
        mode_code = None
        for code, name in ROVER_MODES.items():
            if name == mode_name:
                mode_code = code
                break
        if mode_code is not None:
            rover.set_mode(mode_code)
    elif cmd_type == "command":
        raw_cmd = command.get("command")
        if raw_cmd:
            rover._handle_stdin_command(raw_cmd)
    elif cmd_type == "toggle_mock":
        rover.mock_mode = not rover.mock_mode
        if rover.mock_mode:
            rover.mock_start_time = time.time()
            logger.info("Mock Mode ENABLED")
        else:
            logger.info("Mock Mode DISABLED")

async def mavlink_receive_loop(rover: Rover):
    """Continuously receive MAVLink messages."""
    while True:
        if rover.master:
            try:
                msg = rover.master.recv_match(blocking=False)
                if msg:
                    rover._handle_mavlink_message(msg)
            except Exception as e:
                logger.error(f"Error in MAVLink receive loop: {e}")
        await asyncio.sleep(0.01)

async def run_nats_bridge(rover: Rover, nats_url="nats://localhost:4222"):
    """Run the MAVLink to NATS bridge."""
    logger.info("Connecting to NATS...", extra={'system': 'NATS'})
    nc = await nats.connect(nats_url)
    logger.info(f"Connected to NATS at {nats_url}", extra={'system': 'NATS'})
    
    await asyncio.gather(
        telemetry_publish_loop(rover, nc),
        command_subscribe_loop(rover, nc),
        mavlink_receive_loop(rover)
    )

def main():
    """Main function with command line options for rover control."""
    parser = argparse.ArgumentParser(description='Rover Control Script')
    parser.add_argument('--arm', action='store_true', help='Arm the rover')
    parser.add_argument('--disarm', action='store_true', help='Disarm the rover')
    parser.add_argument('--status', action='store_true', help='Check rover status')
    parser.add_argument('--mode', type=str, help='Set rover mode (e.g., MANUAL, GUIDED, HOLD)')
    parser.add_argument('--monitor', type=int, metavar='SECONDS', help='Monitor for errors for specified seconds')
    parser.add_argument('--test', action='store_true', help='Run full test sequence (arm, monitor, disarm)')
    parser.add_argument('--params', action='store_true', help='Read all parameters from rover')
    parser.add_argument('--param', type=str, metavar='PARAM_NAME', help='Read specific parameter (e.g., RCMAP_THROTTLE)')
    parser.add_argument('--rc-params', action='store_true', help='Read and display RC mapping and channel parameters')
    parser.add_argument('--gps', action='store_true', help='Get GPS position from rover')
    parser.add_argument('--serve', action='store_true', help='Start persistent MAVLink server with JSON telemetry')
    parser.add_argument('--web', action='store_true', help='Start FastAPI webserver with WebSocket support')
    parser.add_argument('--mock', action='store_true', help='Start in Mock Mode (simulated data)')
    parser.add_argument('--port', type=int, default=8000, help='Port for the webserver')
    parser.add_argument('--host', type=str, default='0.0.0.0', help='Host for the webserver')
    parser.add_argument('--connection', type=str, default='/dev/ttyAMA0', help='MAVLink connection string')
    
    args = parser.parse_args()
    
    # If no arguments provided, show help
    if not any([args.arm, args.disarm, args.status, args.mode, args.monitor, args.test, args.params, args.param, args.rc_params, args.gps, args.serve, args.web]):
        parser.print_help()
        return
    
    logger.info("=== Rover Control Script ===")
    
    try:
        rover = Rover(args.connection)
        
        # Try to connect
        if not rover.connect():
            if args.mock:
                logger.info("Connection failed, but --mock specified. Proceeding in Mock Mode.")
            else:
                logger.warning("Failed to connect to rover - check connection string")
        
        # Enable mock mode if requested
        if args.mock:
            rover.mock_mode = True
            rover.mock_start_time = time.time()
            logger.info("Mock Mode ENABLED via command line")

        # Determine if we should run the bridge
        if args.web or args.serve:
            nats_url = f"nats://{args.host}:4222" if args.host != '0.0.0.0' else "nats://localhost:4222"
            try:
                asyncio.run(run_nats_bridge(rover, nats_url))
            except KeyboardInterrupt:
                logger.info("Bridge stopped by user")
            return 0

        try:
            # Check status
            if args.status:
                rover.check_armed("Checking rover status")
                rover.get_current_mode()
            
            # Set mode
            if args.mode:
                mode_upper = args.mode.upper()
                mode_code = None
                for code, name in ROVER_MODES.items():
                    if name == mode_upper:
                        mode_code = code
                        break
                
                if mode_code is not None:
                    success = rover.set_mode(mode_code)
                    if success:
                        logger.info(f"Successfully set mode to {mode_upper}")
                    else:
                        logger.error(f"Failed to set mode to {mode_upper}")
                else:
                    logger.error(f"Unknown mode: {args.mode}. Available modes: {list(ROVER_MODES.values())}")
            
            # Arm rover
            if args.arm:
                logger.info("Arming rover...")
                success = rover.arm()
                if success:
                    logger.info("Rover armed successfully")
                else:
                    logger.error("Failed to arm rover")
            
            # Disarm rover
            if args.disarm:
                logger.info("Disarming rover...")
                success = rover.disarm()
                if success:
                    logger.info("Rover disarmed successfully")
                else:
                    logger.error("Failed to disarm rover")
            
            # Monitor for errors
            if args.monitor:
                logger.info(f"Monitoring for errors for {args.monitor} seconds...")
                error_count, warning_count = rover.monitor_errors(duration=args.monitor)
                logger.info(f"Monitoring complete - Errors: {error_count}, Warnings: {warning_count}")
            
            # Read all parameters
            if args.params:
                logger.info("Reading all parameters...")
                parameters = rover.read_parameters(write_to_file=True)
                if parameters:
                    logger.info(f"Successfully read {len(parameters)} parameters")
                else:
                    logger.error("Failed to read parameters")
            
            # Read specific parameter
            if args.param:
                logger.info(f"Reading specific parameter: {args.param}")
                parameters = rover.read_specific_parameters([args.param])
                if parameters:
                    logger.info(f"Successfully read parameter {args.param}")
                else:
                    logger.error(f"Failed to read parameter {args.param}")
            
            # Read RC parameters
            if args.rc_params:
                logger.info("Reading RC parameters...")
                parameters = rover.read_rc_parameters(write_to_file=True)
                if parameters:
                    logger.info("Successfully read RC parameters")
                else:
                    logger.error("Failed to read RC parameters")
            
            # Get GPS position
            if args.gps:
                logger.info("Getting GPS position...")
                success = rover.get_gps_position()
                if success:
                    logger.info("Successfully retrieved GPS position")
                else:
                    logger.error("Failed to retrieve GPS position")
            
        finally:
            # Always close the connection
            rover.close()
            
    except Exception as e:
        logger.critical(f"Rover operation failed with error: {e}")
        return 1
    
    return 0

if __name__ == '__main__':
    main()
