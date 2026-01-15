# Testing Guide

## Test Cycle

Use the following commands in order to test the system:
1. **Test Build**: Verify local and cross-compilation works.
2. **Test Functionality**: Run unit tests in `test/local_test.go`.
3. **Security Review**: Search for key leaks in logs and code.
4. **Test Deployment**: Deploy to a staging or test target.
5. **Test Live System**: Run integration tests in `test/remote_rover_test.go`.

## UI Testing & Screenshots

**Run UI Tests:**
```bash
go test -v ./test/ui_screenshot_test.go
```