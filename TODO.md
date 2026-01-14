# Integrate .env variable for tsnet API key
1. I added a variable to the .env file for the tsnet API key
2. integrate using it in the cli provision command if it is available. do not log it
3. provision a new key AUTHKEY via the CLI before each deploy by adding it as an automated step  when building via build.ps1 or build.sh or dialtone cli build

# Improve testing
0. create branch called improve-testing
1. improve UI tests to use aria labels after adding them to the web ui
2. send a nats message and recieve feedback in a test all via chromedp and aria labels to get the elements

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