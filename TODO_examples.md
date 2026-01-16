# Create documentation for a concept of a plugin system
1. keep it simple. it should really just be a basic code template that serves as an example for how to create a plugin.
2. do not try to use any dynamic loading of code or anything complex like that.
3. it should probably be able to load, build and run a plugin on the remote robot
4. these plugins should only communicate via the nats bus

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

# add github cli to items that get installed with the `dialtone install` command
1. here is an example of installing on linux
```
(type -p wget >/dev/null || (sudo apt update && sudo apt install wget -y)) \
	&& sudo mkdir -p -m 755 /etc/apt/keyrings \
	&& out=$(mktemp) && wget -nv -O$out https://cli.github.com/packages/githubcli-archive-keyring.gpg \
	&& cat $out | sudo tee /etc/apt/keyrings/githubcli-archive-keyring.gpg > /dev/null \
	&& sudo chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg \
	&& sudo mkdir -p -m 755 /etc/apt/sources.list.d \
	&& echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
	&& sudo apt update \
	&& sudo apt install gh -y
```