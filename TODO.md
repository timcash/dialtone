# Break up README into `./docs` directory and files
1. use the files already in there to guide you.
# Improve Logging
1. use this logging format in all the goland code, you may need to research how to get the function name and line number that the log was called at
```
[iso-8601-timestamp | log level | filename:parent-function-name:line-number] <message>
```
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
3. set the hostname
4. add a robot user