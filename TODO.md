# add mavlink capabilities to dialtone
0. the previous LLM agent broke so you are picking up where it left off
1. read the main `README.md`
2. reed the `docs/develop.md`
3. use `mavlink/rover.py` as a reference for how to implement mavlink capabilities into golang
6. implement only receiving mavlink heatbeat 
7. attemp to deploy to the robot and test it
8. show the heartbeat messages in the web ui passed via a nats subject
9. to debug you can also send ssh commands to the rover via the dialtone cli and it will pull details from the .env file
10. make sure to use the project logger at `src/logger.go`


# Create plugin system
1. keep it simple. it should really just be a basic code template that serves as an example for how to create a plugin.
2. do not try to use any dynamic loading of code or anything complex like that.

# Add mavlink plugin
1. it should be able to flash the flight controller with a new binary
2. it should be able to send mavlink messages to the flight controller
3. it should be able to receive mavlink messages from the flight controller and publish them to the nats bus

# turn current camera code into plugin

# Add geospatial plugin

# Add meshcore plugin

# Add tools to setup a fresh raspberry pi 
1. with the correct SSID and password to join wifi
2. turn off blue tooth with the config file
```
Check the presence of the parameters enable_uart=1 and dtoverlay=pi 3-disable-bt in the file /boot/config.txt by running the following command on the Raspberry Pi:

 cat /boot/config.txt | grep -E "^enable_uart=.|^dtoverlay=pi3-disable-bt"
 ```
3. set the hostname
4. add a robot user

# Add aria labels to the web ui
1. use aria labels for all automated testing via chromedp