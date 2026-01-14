# Testing Guide

## Test Cycle

Use the following commands in order to test the system:
1. **Test Build**: Verify local and cross-compilation works.
2. **Test Functionality**: Run unit tests in `test/local_test.go`.
3. **Security Review**: Search for key leaks in logs and code.
4. **Test Deployment**: Deploy to a staging or test target.
5. **Test Live System**: Run integration tests in `test/remote_rover_test.go`.

## UI Testing & Screenshots

Dialtone includes an automated UI testing suite using `chromedp`. This allows for visual regression testing by capturing screenshots of the live dashboard.

**Run UI Tests:**
```bash
go test -v ./test/ui_screenshot_test.go
```

Screenshots are saved to `test/screenshots/` and include:
- `initial_load.png`: The dashboard immediately after loading.
- `before_send.png`: State after filling in NATS message inputs.
- `after_send.png`: Confirmation after successfully sending a message.