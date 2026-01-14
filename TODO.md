# add mavlink capabilities to dialtone
0. start a new branch `git checkout -b feature/mavlink-upgrade`
1. read the main `README.md`
2. read the `docs/develop.md`
3. use `mavlink/rover.py` as a reference for how to implement mavlink capabilities into golang `src/mavlink.go`
4. implement the ability to send the arm command and get back any warnings or errors for example "rc controller not connected"
5. add a section to the UI `src/web/src/main.ts` and `src/web/index.html` to show mavlink status
5. attemp to deploy to the robot and test it
8. show the mavlink messages in the web ui passed via a nats subject that get recieved during the arming process
9. to debug you can also send ssh commands to the rover via the dialtone cli and it will pull details from the .env file
10. make sure to use the project logger at `src/logger.go`
11. the rover ssh is `tim@192.168.4.36` with password `password` as per the .env file


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