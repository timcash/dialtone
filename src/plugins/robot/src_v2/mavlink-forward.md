# Plan: MAVLink 1-Second Forward Pulse

This plan outlines the implementation of a "Pulse Fwd" button in the Robot src_v2 UI that sends a MAVLink command to drive the rover at half-throttle for 1 second.

## 1. UI Modification (Frontend)

**File:** `src/plugins/robot/src_v2/ui/src/components/three/index.ts`

Add a new button definition to the `Control` mode registry in the `ThreeControl` component.

```typescript
registerButtons('three', ['Control'], {
  'Control': [
    { label: 'Arm', action: () => sendCommand('arm') },
    { label: 'Disarm', action: () => sendCommand('disarm') },
    { label: 'Manual', action: () => sendCommand('mode', 'manual') },
    { label: 'Guided', action: () => sendCommand('mode', 'guided') },
    { label: 'Pulse Fwd', action: () => sendCommand('pulse_fwd') }, // New Button
    null, null, null
  ]
});
```

## 2. NATS Consumer Update (Backend Bridge)

**File:** `src/plugins/mavlink/src_v1/cmd/main.go`

Update the `startRoverCommandConsumer` function to handle the `pulse_fwd` command string. We use a goroutine to avoid blocking the main NATS consumer during the 1-second wait.

```go
		case "pulse_fwd":
			go func() {
				if err := svc.PulseForward(); err != nil {
					logs.Error("rover.command pulse_fwd failed: %v", err)
				}
			}()
```

## 3. MAVLink Implementation (Backend Service)

**File:** `src/plugins/mavlink/app/mavlink.go`

Implement the `PulseForward` method using `RC_CHANNELS_OVERRIDE`. This assumes Channel 3 is the throttle (ArduRover default).

```go
// PulseForward sends a 1-second half-throttle command (1750 PWM) then returns to neutral (1500 PWM)
func (s *MavlinkService) PulseForward() error {
	logs.Info("MavlinkService: Pulsing forward for 1s (PWM 1750)")

	// Start Pulse (1750 = ~50% forward)
	err := s.node.WriteMessageAll(&common.MessageRcChannelsOverride{
		TargetSystem:    0,
		TargetComponent: 0,
		Chan3Raw:        1750, 
	})
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)

	// Return to Neutral (1500 = Stop)
	return s.node.WriteMessageAll(&common.MessageRcChannelsOverride{
		TargetSystem:    0,
		TargetComponent: 0,
		Chan3Raw:        1500,
	})
}
```

## 4. Verification Strategy

1. **Local Mock Test:**
   - Run `robot src_v2 test` to ensure UI builds and the button appears in the `Three` section.
   - Use NATS sub to verify `rover.command` receives `{"cmd":"pulse_fwd"}` when the button is clicked.

2. **MAVLink Mock Test:**
   - Run the MAVLink bridge in mock mode.
   - Verify log output shows "Pulsing forward for 1s".

3. **Rover Integration:**
   - Deploy to `rover` node.
   - Verify vehicle responds in `MANUAL` or `GUIDED` mode.
