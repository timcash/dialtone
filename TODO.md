# FEATURE: `dialtone-dev cli tool`
1. create basic functions to match the descriptions in agent.md
2. start with the `dialtone-dev plan` command that makes a template plan for editing in the `plan` directory
3. after that command works look at agent.md again and add more steps to the already created plan. 
4. rember to follow developmet steps like creating a branch

# FEATURE: `improve-cli-build-commands`
1. use simple commands like deploy, install, build, dev
2. add options and a help commands for each
3. start by focusing on build commands

# FEATURE: `linux-wsl-camera-support`
0. Allow development on Linux/WSL with camera enabled. create a `plan-linux-wsl-camera-support.md` file in the `plan` directory to track progress
1. start a new branch `git checkout -b feature/linux-wsl-camera-support`
2. read the main `README.md` and `docs/develop.md` for the TDD loop and build commands
3. implement a new CLI command `install-local-deps --linux-wsl` in `src/manager.go` to install Go 1.25.5 and Node.js on the local Ubuntu/WSL system
4. modify `RunBuild` in `src/manager.go` to support a native build on Linux (without Podman) when a `-local` flag is provided or if Podman is missing
5. ensure the build enables CGO (`CGO_ENABLED=1`) to support the `go4vl` camera library in `src/camera_linux.go`
6. create a verification test `test/wsl_camera_test.go` that attempts to compile the project and checks for the presence of V4L2 headers
7. use the project logger `dialtone.LogInfo` from `src/logger.go` for all status updates
8. to debug, you can use `test/camera_diag_test.go` which already implements basic camera opening and frame capture
9. verify the build works by running the newly created test and performing a `dialtone full-build`
10. update `docs/cli.md` to include the new `install-local-deps --linux-wsl` command and WSL development instructions.
11. integrate any other build docs into `docs/cli.md`
12. remove any old build docs from `docs/develop.md` and any old build scripts
13. you are using a WSL environment to develop on Linux Ubuntu
14. finish by creating a pull request
15. remember to create a `plan-linux-wsl-camera-support.md` file in the `plan` directory to track progress

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
11. the rover ssh is `tim@192.168.4.36` with password `password` if needed for debuging

# allow development on the remote system
NOTE the previous LLM crashed while working on this
0. create a branch `git checkout -b feature/remote-development`
1. read the main `README.md`
2. read the `docs/develop.md`
3. implement the following features to allow development directly on the remote robot
1. currently we develop on a windows machine and build before deploying to the remote robot
2. we should be able to use the dialtone cli to send remote commmands so we can build and run development code there directly
3. this will require setting up a golang environment on the remote robot
4. so the robot will need `npm` and `go` installed
5. make sure there are cli commands to copy all the code to the remote robot for development
6. make sure there are cli commands to build the code on the remote robot for development including the web code
7. make sure there are cli commands to install golang and npm on the remote robot for development
9. to debug you can also send ssh commands to the rover via the dialtone cli and it will pull details from the .env file
10. make sure to use the project logger at `src/logger.go`
11. the rover ssh is `tim@192.168.4.36` with password `password` if needed for debuging

it created these things you need to test

       * install-deps: Installs Go 1.25.5 and Node.js on the remote robot.
       * sync-code: Syncs source code (Go + Web) to the remote robot.
       * remote-build: Builds the project (Web + Go) directly on the remote robot.


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