"""Offline installer integration tests. Never execute the fixture binaries.

python tests/installers.py [--powershell powershell|pwsh] [--shell /path/to/sh]
"""

import argparse
import hashlib
import http.server
import os
from pathlib import Path
import platform
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import unittest


ROOT = Path(__file__).resolve().parents[1]
ASSETS = (
    "dialtone-linux-x86_64",
    "dialtone-linux-arm64",
    "dialtone-macos-arm64",
    "dialtone-windows-x86_64.exe",
)
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("--powershell", default="pwsh")
parser.add_argument("--shell", help="Run Unix selection/installer tests, even on Windows")
parser.add_argument("--live-release", action="store_true", help="Opt in to a real HTTPS release download; never executes it")
OPTIONS, UNITTEST_ARGS = parser.parse_known_args()
UNIX = os.name != "nt" or bool(OPTIONS.shell)


def shell_path(path):
    value = str(path)
    if os.name == "nt" and UNIX:
        value = Path(path).as_posix()
        if len(value) > 2 and value[1] == ":":
            value = "/" + value[0].lower() + value[2:]
    return value


class Fixture(http.server.BaseHTTPRequestHandler):
    files = {}
    requests = []
    redirects = {}
    truncated = set()

    def do_GET(self):
        self.requests.append(self.path)
        if self.path in self.redirects:
            self.send_response(302)
            self.send_header("Location", self.redirects[self.path])
            self.end_headers()
            return
        body = self.files.get(self.path)
        self.send_response(200 if body is not None else 404)
        self.send_header("Content-Length", str(len(body or b"") + (10 if self.path in self.truncated else 0)))
        self.end_headers()
        if body:
            self.wfile.write(body)
        self.close_connection = True

    def log_message(self, *_args):
        pass


class Installers(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Fixture)
        cls.thread = threading.Thread(
            target=lambda: cls.server.serve_forever(poll_interval=0.05), daemon=True
        )
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def setUp(self):
        # All generated state is inside this worktree, including installer temps.
        self.temp = tempfile.TemporaryDirectory(prefix=".installer-tests-", dir=ROOT)
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.home = self.root / "home with spaces"
        self.home.mkdir()
        self.tmp = self.root / "temp with spaces"
        self.tmp.mkdir()
        self.destination = self.home / "custom tools"
        self.ps_injection = ""
        self.ps_prefix = ""
        self.ps_finally = ""
        self.env = os.environ.copy()
        # Let each PowerShell edition discover its own standard modules instead
        # of inheriting the hosting edition's module paths.
        self.env.pop("PSModulePath", None)
        for key in list(self.env):
            if key.startswith("DIALTONE_"):
                del self.env[key]
        self.env.update(
            HOME=str(self.home),
            USERPROFILE=str(self.home),
            TMPDIR=str(self.tmp),
            TMP=str(self.tmp),
            TEMP=str(self.tmp),
            DIALTONE_STATE_DIR=str(self.home / "node state"),
            DIALTONE_RELEASE_BASE_URL=f"http://127.0.0.1:{self.server.server_port}/releases",
            NO_PROXY="127.0.0.1",
            no_proxy="127.0.0.1",
        )
        if UNIX:
            self.env["HOME"] = shell_path(self.home)
            self.env["TMPDIR"] = shell_path(self.tmp)
        Fixture.files = {}
        Fixture.requests = []
        Fixture.redirects = {}
        Fixture.truncated = set()
        self.payloads = {}
        for version in ("v0.5.3", "v1.2.3-rc.1"):
            for asset in ASSETS:
                # Deliberately inert payloads, never real Dialtone nodes.
                body = f"#!/bin/sh\nexit 91\n# {version} {asset}\n".encode()
                self.payloads[version, asset] = body
                path = f"/releases/download/{version}/{asset}"
                Fixture.files[path] = body
                Fixture.files[path + ".sha256"] = (
                    f"{hashlib.sha256(body).hexdigest()}  {asset}\n".encode()
                )
        self.os = platform.system()
        self.arch = platform.machine().lower()
        self.shim = self.root / "shim tools"
        self.shim.mkdir()
        self.env["DIALTONE_TEST_TOOL_PATH"] = shell_path(self.shim)
        if UNIX:
            self.driver = OPTIONS.shell or shutil.which("sh")
            if not self.driver:
                self.fail("sh is required")
            self.asset = self.native_unix_asset()
        else:
            self.driver = shutil.which(OPTIONS.powershell)
            if not self.driver:
                self.fail(f"{OPTIONS.powershell} is required; do not silently skip native coverage")
            self.asset = "dialtone-windows-x86_64.exe"

    def native_unix_asset(self):
        if self.os == "Linux" and self.arch in ("x86_64", "amd64"):
            return "dialtone-linux-x86_64"
        if self.os == "Linux" and self.arch in ("aarch64", "arm64"):
            return "dialtone-linux-arm64"
        if self.os == "Darwin" and self.arch in ("aarch64", "arm64"):
            return "dialtone-macos-arm64"
        if OPTIONS.shell:
            self.simulate("Linux", "x86_64")
            return "dialtone-linux-x86_64"
        self.fail(f"No native release asset for {self.os}/{self.arch}")

    def simulate(self, system, arch, apple_silicon="0"):
        # Fake selection only, never pretend the downloaded fixture is a native binary.
        for name, content in (
            ("uname", f'case "$1" in -s) echo {system};; -m) echo {arch};; *) exit 1;; esac'),
            ("sysctl", f"echo {apple_silicon}"),
        ):
            script = self.shim / name
            script.write_text("#!/bin/sh\n" + content + "\n", encoding="utf-8", newline="\n")
            script.chmod(0o755)

    @property
    def installed(self):
        return self.destination / ("dialtone" if UNIX else "dialtone.exe")

    @property
    def asset_path(self):
        return f"/releases/download/v0.5.3/{self.asset}"

    def run_installer(self, *args, ok=True, default_dir=False, piped=False, error=None, recovery=False, timeout=45):
        if UNIX:
            # Git's bin/sh.exe launcher prepends its own tools to PATH. Put the
            # selection shims first inside the shell, after that launcher runs.
            command = [self.driver, "-c", 'export PATH="$DIALTONE_TEST_TOOL_PATH:$PATH"; exec sh "$@"', "installer-test"]
            if piped:
                command += ["-s", "--"]
            else:
                command += [shell_path(ROOT / "install.sh")]
            if not default_dir:
                command += ["--install-dir", shell_path(self.destination)]
        else:
            command = [self.driver, "-NoLogo", "-NoProfile", "-NonInteractive"]
            if piped:
                command += ["-Command", "$input | Out-String | Invoke-Expression"]
                self.env["DIALTONE_INSTALL_DIR"] = str(self.destination)
            else:
                # Exercise the README's in-memory script form, also on hosts
                # whose local-file execution policy is Restricted. No policy changes.
                def quote(value):
                    return "'" + str(value).replace("'", "''") + "'"
                script = ROOT / "install.ps1"
                if self.ps_injection:
                    # Inject local operation failures at function boundaries.
                    # The production installer has no test environment switches.
                    source = script.read_text(encoding="utf-8")
                    before, after = source.rsplit("\nInstall-Dialtone", 1)
                    script = self.root / "injected.ps1"
                    script.write_text(before + "\n" + self.ps_injection + "\nInstall-Dialtone" + after, encoding="utf-8")
                invocation = "& ([ScriptBlock]::Create([IO.File]::ReadAllText(" + quote(script) + ")))"
                if not default_dir:
                    invocation += " -InstallDir " + quote(self.destination)
                invocation += " " + " ".join(quote(arg) if not arg.startswith("-") else arg for arg in args)
                if self.ps_finally:
                    invocation = "try { " + invocation + " } finally { " + self.ps_finally + " }"
                invocation = self.ps_prefix + invocation
                command += ["-Command", invocation]
        if UNIX or piped:
            command += list(args)
        source = None
        if piped:
            source = (ROOT / ("install.sh" if UNIX else "install.ps1")).read_bytes()
        result = subprocess.run(
            command, cwd=self.root, env=self.env, input=source,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout,
        )
        output = result.stdout.decode(errors="replace")
        self.assertEqual(result.returncode == 0, ok, output)
        if error:
            self.assertIn(error.lower(), output.lower(), output)
        self.assertFalse((self.home / "node state").exists(), output)
        self.assertFalse((self.home / ".dialtone").exists(), output)
        self.assertEqual(list(self.tmp.iterdir()), [], output)
        if self.destination.is_dir() and not recovery:
            self.assertEqual(list(self.destination.glob(".dialtone-install*")), [], output)
        return output

    def old_install(self):
        self.destination.mkdir(exist_ok=True)
        self.installed.write_bytes(b"previous executable")

    def assert_preserved(self):
        self.assertEqual(self.installed.read_bytes(), b"previous executable")

    def test_native_install_and_repeat_with_spaces(self):
        self.run_installer()
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])
        if UNIX and os.name != "nt":
            self.assertTrue(self.installed.stat().st_mode & 0o111)
        self.run_installer()
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])
        self.assertEqual(Fixture.requests, [self.asset_path, self.asset_path + ".sha256"] * 2)

    def test_piped_bootstrap(self):
        self.run_installer(piped=True)
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])

    @unittest.skipUnless(UNIX, "curl | sh truncation guard")
    def test_truncated_pipe_does_not_install(self):
        source = (ROOT / "install.sh").read_text(encoding="utf-8").split('\nmain "$@"')[0]
        result = subprocess.run(
            [self.driver], input=source.encode(), env=self.env, cwd=self.root,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=10,
        )
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertEqual(Fixture.requests, [])
        self.assertFalse(self.destination.exists())

    def test_environment_defaults_and_explicit_option_precedence(self):
        self.env["DIALTONE_INSTALL_DIR"] = shell_path(self.home / "unused")
        self.env["DIALTONE_VERSION"] = "v9.9.9"
        args = ["--version", "v1.2.3-rc.1"] if UNIX else ["-Version", "v1.2.3-rc.1"]
        self.run_installer(*args)
        self.assertEqual(self.installed.read_bytes(), self.payloads["v1.2.3-rc.1", self.asset])
        self.assertFalse((self.home / "unused").exists())
        self.assertTrue(all("/v1.2.3-rc.1/" in path for path in Fixture.requests))

    def test_default_directory(self):
        # PowerShell's automatic HOME comes from the user profile, not env:HOME.
        # A launcher explicitly sets it to our isolated test home.
        if not UNIX:
            quoted = str(self.home).replace("'", "''")
            script = str(ROOT / "install.ps1").replace("'", "''")
            invocation = f"Set-Variable HOME '{quoted}' -Force; & ([ScriptBlock]::Create([IO.File]::ReadAllText('{script}')))"
            result = subprocess.run(
                [self.driver, "-NoProfile", "-NonInteractive", "-Command", invocation],
                env=self.env, cwd=self.root, capture_output=True, timeout=45,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.run_installer(default_dir=True)
        target = self.home / ".local" / "bin" / ("dialtone" if UNIX else "dialtone.exe")
        self.assertEqual(target.read_bytes(), self.payloads["v0.5.3", self.asset])
        self.assertEqual(list(self.tmp.iterdir()), [])

    def test_custom_directory_environment(self):
        self.env["DIALTONE_INSTALL_DIR"] = shell_path(self.destination)
        self.env["DIALTONE_VERSION"] = "v1.2.3-rc.1"
        self.run_installer(default_dir=True)
        self.assertEqual(self.installed.read_bytes(), self.payloads["v1.2.3-rc.1", self.asset])

    def test_relative_directory_with_spaces(self):
        self.destination = self.root / "relative tools"
        self.env["DIALTONE_INSTALL_DIR"] = "relative tools"
        self.run_installer(default_dir=True)
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])

    @unittest.skipIf(UNIX, "PowerShell provider location")
    def test_provider_location_differs_from_process_directory(self):
        provider_dir = self.root / "provider location"
        provider_dir.mkdir()
        self.destination = provider_dir / "relative tools"
        self.env["DIALTONE_INSTALL_DIR"] = "relative tools"
        quoted = str(provider_dir).replace("'", "''")
        self.ps_prefix = f"Set-Location -LiteralPath '{quoted}'; "
        self.run_installer(default_dir=True)
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])
        self.assertFalse((self.root / "relative tools").exists())

    @unittest.skipIf(UNIX, "PowerShell provider validation")
    def test_non_filesystem_install_directory_rejected(self):
        self.ps_prefix = "Set-Location Env:; "
        self.env["DIALTONE_INSTALL_DIR"] = "DIALTONE_NOT_A_DIRECTORY"
        self.run_installer(default_dir=True, ok=False, error="must use the FileSystem provider")
        self.assertEqual(Fixture.requests, [])

    def test_default_versions_match_scripts_and_docs(self):
        sh = (ROOT / "install.sh").read_text(encoding="utf-8")
        ps = (ROOT / "install.ps1").read_text(encoding="utf-8")
        shell_default = re.search(r'version=\$\{DIALTONE_VERSION:-(v[^}]+)\}', sh).group(1)
        windows_default = re.search(r"DIALTONE_VERSION } else { '(v[^']+)'", ps).group(1)
        self.assertEqual(shell_default, windows_default)
        self.assertIn(f'Install **{shell_default}**', (ROOT / "README.md").read_text(encoding="utf-8"))
        self.assertIn(f'default to **{shell_default}**', (ROOT / "docs/install.md").read_text(encoding="utf-8"))
        self.assertIn((shell_default, self.asset), self.payloads)

    @unittest.skipUnless(UNIX and os.name != "nt", "Unix filenames with backslash/newline")
    def test_odd_temp_directory_name(self):
        self.tmp = self.root / "temp\\with\nnewline"
        self.tmp.mkdir()
        self.env["TMPDIR"] = str(self.tmp)
        self.run_installer()
        self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])

    def legacy_tls_setup(self):
        self.ps_prefix = "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls; $beforeTls = [Net.ServicePointManager]::SecurityProtocol; "
        self.ps_finally = "if ([Net.ServicePointManager]::SecurityProtocol -ne $beforeTls) { throw 'TLS state leaked' }; Write-Host 'TLS_STATE_RESTORED'"

    @unittest.skipIf(UNIX, "PowerShell process TLS settings")
    def test_tls_state_restored_on_success_and_failure(self):
        self.legacy_tls_setup()
        self.assertIn("TLS_STATE_RESTORED", self.run_installer())
        Fixture.files[self.asset_path] = b"bad binary"
        self.assertIn("TLS_STATE_RESTORED", self.run_installer(ok=False, error="SHA-256 mismatch"))

    def readme_bootstrap(self):
        # The exact README Windows command, pointed at the fixture copy of install.ps1.
        source_url = "https://raw.githubusercontent.com/timcash/dialtone/main/install.ps1"
        readme = (ROOT / "README.md").read_text(encoding="utf-8")
        bootstrap = re.search(r"```powershell\n(.*?)\n```", readme, re.S).group(1)
        self.assertEqual(bootstrap.count(source_url), 1, bootstrap)
        self.assertNotIn("\n", bootstrap.strip())
        self.env["DIALTONE_INSTALL_DIR"] = str(self.destination)
        return bootstrap.replace(source_url, f"http://127.0.0.1:{self.server.server_port}/bootstrap/install.ps1")

    def run_bootstrap(self, command):
        result = subprocess.run(
            [self.driver, "-NoProfile", "-NonInteractive", "-Command", command],
            cwd=self.root, env=self.env, capture_output=True, timeout=45,
        )
        output = (result.stdout + result.stderr).decode(errors="replace")
        self.assertFalse((self.home / "node state").exists(), output)
        self.assertEqual(list(self.tmp.iterdir()), [], output)
        return result.returncode, output

    @unittest.skipIf(UNIX, "README PowerShell bootstrap")
    def test_readme_bootstrap_and_tls_restoration(self):
        Fixture.files["/bootstrap/install.ps1"] = (ROOT / "install.ps1").read_bytes()
        bootstrap = self.readme_bootstrap()
        self.legacy_tls_setup()
        command = self.ps_prefix + "try { " + bootstrap + " } finally { " + self.ps_finally + " }"
        for success in (True, False):
            with self.subTest(bootstrap_download=success):
                if not success:
                    del Fixture.files["/bootstrap/install.ps1"]
                code, output = self.run_bootstrap(command)
                self.assertEqual(code == 0, success, output)
                self.assertIn("TLS_STATE_RESTORED", output)
                self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", self.asset])

    @unittest.skipIf(UNIX, "README PowerShell bootstrap truncation")
    def test_readme_bootstrap_truncated_script_does_not_install(self):
        source = (ROOT / "install.ps1").read_bytes()
        cuts = {
            "mid-function": source[: source.index(b"function Install-Dialtone") + 200],
            "before-call": source[: source.rindex(b"\nInstall-Dialtone")],
        }
        bootstrap = self.readme_bootstrap()
        for label, body in cuts.items():
            with self.subTest(cut=label):
                # The server promises more bytes than it sends: curl reports a
                # partial transfer after writing what it received.
                Fixture.files["/bootstrap/install.ps1"] = body
                Fixture.truncated = {"/bootstrap/install.ps1"}
                Fixture.requests = []
                code, output = self.run_bootstrap(bootstrap)
                if label == "mid-function":
                    self.assertNotEqual(code, 0, output)
                self.assertFalse(self.installed.exists(), output)
                self.assertEqual([p for p in Fixture.requests if p.startswith("/releases/")], [], output)

    @unittest.skipUnless(OPTIONS.live_release, "Opt-in HTTPS release smoke test")
    def test_live_release_https(self):
        del self.env["DIALTONE_RELEASE_BASE_URL"]
        if UNIX:
            self.run_installer(timeout=330)
        else:
            self.legacy_tls_setup()
            self.assertIn("TLS_STATE_RESTORED", self.run_installer(timeout=330))
        self.assertGreater(self.installed.stat().st_size, 1024 * 1024)
        self.assertEqual(Fixture.requests, [])
        # Independently re-check the installed bytes against the published record.
        import urllib.request
        url = f"https://github.com/timcash/dialtone/releases/download/v0.5.3/{self.asset}.sha256"
        with urllib.request.urlopen(url, timeout=60) as response:
            record = response.read().decode("ascii")
        self.assertEqual(record, f"{hashlib.sha256(self.installed.read_bytes()).hexdigest()}  {self.asset}\n")
        # Only after verification: --help, which reads at most DIALTONE_STATE_DIR/config.json
        # (absent here) and starts nothing.
        result = subprocess.run(
            [str(self.installed), "--help"], cwd=self.root, env=self.env,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=30,
        )
        output = result.stdout.decode(errors="replace")
        self.assertEqual(result.returncode, 0, output)
        self.assertIn("usage: dialtone", output)
        self.assertFalse((self.home / "node state").exists(), output)
        self.assertFalse((self.home / ".dialtone").exists(), output)

    def test_corrupt_binary_preserves_previous(self):
        self.old_install()
        Fixture.files[self.asset_path] = b"corrupt binary"
        self.run_installer(ok=False, error="SHA-256 mismatch")
        self.assert_preserved()

    def test_missing_binary_preserves_previous(self):
        self.old_install()
        del Fixture.files[self.asset_path]
        self.run_installer(ok=False)
        self.assertEqual(Fixture.requests, [self.asset_path])
        self.assert_preserved()

    def test_missing_checksum_preserves_previous(self):
        self.old_install()
        del Fixture.files[self.asset_path + ".sha256"]
        self.run_installer(ok=False)
        self.assertEqual(Fixture.requests, [self.asset_path, self.asset_path + ".sha256"])
        self.assert_preserved()

    def test_truncated_download_preserves_previous(self):
        self.old_install()
        Fixture.truncated.add(self.asset_path)
        self.run_installer(ok=False)
        self.assertEqual(Fixture.requests, [self.asset_path])
        self.assert_preserved()

    def test_invalid_checksum_records_preserve_previous(self):
        self.old_install()
        checksum = Fixture.files[self.asset_path + ".sha256"]
        variants = {
            "empty": b"",
            "html": b"<html>not a checksum</html>\n",
            "short hash": b"ab12  " + self.asset.encode() + b"\n",
            "non hex": b"g" * 64 + b"  " + self.asset.encode() + b"\n",
            "wrong hash": b"0" * 64 + b"  " + self.asset.encode() + b"\n",
            "wrong filename": checksum.replace(self.asset.encode(), b"other-file"),
            "extra record": checksum + checksum,
            "trailing data": checksum + b"x",
            "no final newline": checksum.rstrip(b"\n"),
            "byte order mark": b"\xef\xbb\xbf" + checksum,
            "embedded null": checksum[:10] + b"\0" + checksum[11:],
        }
        for label, record in variants.items():
            with self.subTest(label=label):
                Fixture.files[self.asset_path + ".sha256"] = record
                self.run_installer(ok=False, error="SHA-256 mismatch" if label == "wrong hash" else "invalid checksum record")
                self.assert_preserved()

    def test_empty_binary_rejected_even_with_matching_hash(self):
        self.old_install()
        Fixture.files[self.asset_path] = b""
        Fixture.files[self.asset_path + ".sha256"] = (
            f"{hashlib.sha256(b'').hexdigest()}  {self.asset}\n".encode()
        )
        self.run_installer(ok=False, error="empty binary")
        self.assert_preserved()

    def test_first_install_failure_leaves_no_executable(self):
        del Fixture.files[self.asset_path + ".sha256"]
        self.run_installer(ok=False)
        self.assertFalse(self.installed.exists())

    def test_missing_version_and_bad_tag(self):
        self.old_install()
        for tag in ("v9.9.9", "latest", "v0.5.3/../v1.2.3", "v0.5.3?query"):
            with self.subTest(tag=tag):
                args = ["--version", tag] if UNIX else ["-Version", tag]
                self.run_installer(*args, ok=False)
                self.assert_preserved()

    def test_destination_directory_is_not_replaced(self):
        self.installed.mkdir(parents=True)
        sentinel = self.installed / "keep"
        sentinel.write_bytes(b"keep")
        self.run_installer(ok=False)
        self.assertEqual(sentinel.read_bytes(), b"keep")
        self.assertEqual(list(self.installed.iterdir()), [sentinel])

    def test_replace_failure_preserves_previous(self):
        self.old_install()
        if UNIX:
            # Simulate a final rename failure (e.g. permissions/filesystem).
            script = self.shim / "mv"
            script.write_text("#!/bin/sh\nexit 93\n", encoding="utf-8", newline="\n")
            script.chmod(0o755)
            self.run_installer(ok=False)
        else:
            # Real Windows sharing lock, not a mocked File.Replace call.
            import ctypes
            from ctypes import wintypes
            create = ctypes.windll.kernel32.CreateFileW
            create.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD,
                               wintypes.LPVOID, wintypes.DWORD, wintypes.DWORD, wintypes.HANDLE]
            create.restype = wintypes.HANDLE
            close = ctypes.windll.kernel32.CloseHandle
            close.argtypes = [wintypes.HANDLE]
            handle = create(str(self.installed), 0x80000000, 0, None, 3, 0, None)
            self.assertNotEqual(handle, wintypes.HANDLE(-1).value)
            try:
                self.run_installer(ok=False)
            finally:
                close(handle)
        self.assert_preserved()

    @unittest.skipIf(UNIX, "Windows partial ReplaceFile failure")
    def test_destructive_partial_replace_rolls_back(self):
        for operation in (
            "[IO.File]::Move($Destination, $Backup)",
            "[IO.File]::Delete($Destination)",
            "[IO.File]::Move($Destination, $Backup); [IO.File]::Copy($Source, $Destination)",
        ):
            with self.subTest(operation=operation):
                self.old_install()
                self.ps_injection = """
function Replace-DialtoneExecutable($Source, $Destination, $Backup) {
    %s
    throw (New-Object ComponentModel.Win32Exception(1176))
}
""" % operation
                self.run_installer(ok=False, error="previous executable restored")
                self.assert_preserved()

    @unittest.skipIf(UNIX, "Windows recovery copy retained after rollback failure")
    def test_failed_rollback_keeps_recovery_and_reports_path(self):
        self.old_install()
        self.ps_injection = """
function Replace-DialtoneExecutable($Source, $Destination, $Backup) {
    [IO.File]::Move($Destination, $Backup)
    throw (New-Object ComponentModel.Win32Exception(1176))
}
function Restore-DialtoneExecutable($Backup, $Destination) {
    throw 'Injected restore failure'
}
"""
        output = self.run_installer(ok=False, error="previous executable kept at", recovery=True)
        backups = list(self.destination.glob(".dialtone-install-*/dialtone.previous.exe"))
        self.assertEqual(len(backups), 1, output)
        self.assertEqual(backups[0].read_bytes(), b"previous executable")
        # PowerShell may wrap the error display; remove whitespace for path comparison.
        self.assertIn("".join(str(backups[0]).split()), "".join(output.split()))
        self.assertIn("Injected restore failure", output)

    @unittest.skipIf(UNIX, "Deterministic native Windows architecture simulation")
    def test_native_arm64_or_unknown_rejected_even_with_amd64_shell(self):
        self.env["PROCESSOR_ARCHITECTURE"] = "AMD64"
        self.env["PROCESSOR_ARCHITEW6432"] = ""
        for architecture in ("'ARM64'", "$null"):
            with self.subTest(machine=architecture):
                self.ps_injection = "function Get-DialtoneMachineArchitecture { " + architecture + " }"
                self.run_installer(ok=False, error="unsupported Windows architecture")
                self.assertEqual(Fixture.requests, [])

    def test_insecure_remote_base_is_rejected_before_download(self):
        self.env["DIALTONE_RELEASE_BASE_URL"] = "http://example.invalid/releases"
        self.run_installer(ok=False, error="release base must use HTTPS")
        self.assertEqual(Fixture.requests, [])

    def test_redirect_to_asset_and_missing_redirect_target(self):
        original = Fixture.files.pop(self.asset_path)
        Fixture.files["/fixture-binary"] = original
        Fixture.redirects[self.asset_path] = "/fixture-binary"
        if UNIX:
            # POSIX permits no redirects from its HTTP-only fixture mode,
            # preventing a local fixture URL from redirecting off loopback.
            self.old_install()
            self.run_installer(ok=False, error="redirect")
            self.assert_preserved()
        else:
            self.run_installer()
        self.old_install()
        Fixture.redirects[self.asset_path] = "/missing"
        self.run_installer(ok=False)
        self.assert_preserved()

    @unittest.skipUnless(UNIX, "Unix target selection simulation")
    def test_simulated_supported_unix_selection(self):
        for system, arch, silicon, asset in (
            ("Linux", "x86_64", "0", "dialtone-linux-x86_64"),
            ("Linux", "aarch64", "0", "dialtone-linux-arm64"),
            ("Darwin", "arm64", "1", "dialtone-macos-arm64"),
            ("Darwin", "x86_64", "1", "dialtone-macos-arm64"),
        ):
            with self.subTest(system=system, arch=arch, rosetta=silicon):
                self.simulate(system, arch, silicon)
                self.run_installer()
                self.assertEqual(self.installed.read_bytes(), self.payloads["v0.5.3", asset])

    def test_unsupported_platforms_before_download(self):
        self.old_install()
        if UNIX:
            variants = [("Darwin", "x86_64"), ("Linux", "armv7l"), ("FreeBSD", "x86_64"), ("MINGW64_NT", "x86_64")]
        else:
            variants = [("AMD64", "ARM64"), ("ARM64", ""), ("x86", "")]
        for first, second in variants:
            with self.subTest(platform=(first, second)):
                if UNIX:
                    self.simulate(first, second)
                else:
                    self.env["PROCESSOR_ARCHITECTURE"] = first
                    self.env["PROCESSOR_ARCHITEW6432"] = second
                self.run_installer(ok=False, error="unsupported")
                self.assert_preserved()
                self.assertEqual(Fixture.requests, [])


if __name__ == "__main__":
    mode = (
        "fixtures plus opt-in live HTTPS download (verified binary runs --help only)"
        if OPTIONS.live_release else "fixtures only, no binary execution"
    )
    print(
        f"Installer tests: {'Unix via ' + (OPTIONS.shell or 'native sh') if UNIX else OPTIONS.powershell} "
        f"on {platform.system()}/{platform.machine()}; {mode}.",
        flush=True,
    )
    unittest.main(argv=[sys.argv[0]] + UNITTEST_ARGS, verbosity=2)
