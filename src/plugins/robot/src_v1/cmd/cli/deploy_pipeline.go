package cli

import (
	"fmt"
	"os"
	"path"
	"strings"

	configv1 "dialtone/dev/plugins/config/src_v1/go"
	logs "dialtone/dev/plugins/logs/src_v1/go"
	ssh_plugin "dialtone/dev/plugins/ssh/src_v1/go"
)

func deployRobot(versionDir string, opts deployOptions) error {
	rt, err := configv1.ResolveRuntime("")
	if err != nil {
		return err
	}
	ctx := &deployContext{
		versionDir: versionDir,
		opts:       opts,
		repoRoot:   rt.RepoRoot,
	}

	if err := ensureRobotAuthKey(ctx.repoRoot); err != nil {
		return err
	}

	logs.Info("[DEPLOY] Connecting to %s...", opts.Host)
	client, err := ssh_plugin.DialSSH(opts.Host, opts.Port, opts.User, opts.Pass)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	steps := []deployStep{
		{name: "validate remote sudo", run: func() error { return validateSudo(client) }},
		{name: "pre-deployment remote resource checks", run: func() error { return checkRemoteResources(client) }},
		{name: "detect remote target", run: func() error {
			goos, goarch, err := detectRemoteTarget(client)
			if err != nil {
				return err
			}
			ctx.goos = goos
			ctx.goarch = goarch
			logs.Info("[DEPLOY] Remote target: %s/%s", goos, goarch)
			return nil
		}},
		{name: "build robot UI", run: func() error { return buildRobotUI(ctx.repoRoot, ctx.versionDir) }},
		{name: "build robot binary", run: func() error {
			bin, err := buildRobotBinary(ctx.repoRoot, ctx.versionDir, ctx.goos, ctx.goarch)
			if err != nil {
				return err
			}
			ctx.localBin = bin
			if _, err := os.Stat(ctx.localBin); err != nil {
				return fmt.Errorf("local binary missing after build (%s): %w", ctx.localBin, err)
			}
			return nil
		}},
		{name: "prepare remote paths", run: func() error {
			ctx.remoteRoot = path.Join("/home", ctx.opts.User, ".dialtone", "robot", ctx.versionDir)
			ctx.remoteBinDir = path.Join(ctx.remoteRoot, "bin")
			ctx.remoteUIDir = path.Join(ctx.remoteRoot, "ui", "dist")
			ctx.remoteBin = path.Join(ctx.remoteBinDir, "robot-src_v1")
			ctx.remoteBinTmp = ctx.remoteBin + ".new"
			_, err := ssh_plugin.RunSSHCommand(client, "mkdir -p "+shellQuote(ctx.remoteBinDir)+" "+shellQuote(ctx.remoteUIDir))
			if err != nil {
				return fmt.Errorf("failed to prepare remote directories: %w", err)
			}
			return nil
		}},
		{name: "upload binary", run: func() error {
			if err := ssh_plugin.UploadFile(client, ctx.localBin, ctx.remoteBinTmp); err != nil {
				return fmt.Errorf("failed to upload binary: %w", err)
			}
			if _, err := ssh_plugin.RunSSHCommand(client, "chmod +x "+shellQuote(ctx.remoteBinTmp)); err != nil {
				return fmt.Errorf("failed to chmod remote binary: %w", err)
			}
			return nil
		}},
		{name: "upload UI dist", run: func() error {
			preset := configv1.NewPluginPreset(rt, "robot", ctx.versionDir)
			if err := uploadDir(client, preset.UIDist, ctx.remoteUIDir); err != nil {
				return fmt.Errorf("failed to upload UI dist: %w", err)
			}
			return nil
		}},
		{name: "artifact smoke checks", run: func() error {
			if _, err := ssh_plugin.RunSSHCommand(client, "test -x "+shellQuote(ctx.remoteBinTmp)); err != nil {
				return fmt.Errorf("remote uploaded binary not executable: %w", err)
			}
			if _, err := ssh_plugin.RunSSHCommand(client, "test -f "+shellQuote(path.Join(ctx.remoteUIDir, "index.html"))); err != nil {
				return fmt.Errorf("remote ui dist missing index.html: %w", err)
			}
			return nil
		}},
		{name: "prune conflicting tailscale nodes", run: func() error {
			hostname := strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
			if hostname == "" {
				hostname = "drone-1"
			}
			return pruneTailscaleNodes(hostname)
		}},
		{name: "install/swap robot service", run: func() error {
			if ctx.opts.Service {
				if err := setupRemoteRobotService(client, ctx.opts, ctx.versionDir); err != nil {
					return err
				}
				if _, err := ssh_plugin.RunSSHCommand(client, "mv "+shellQuote(ctx.remoteBinTmp)+" "+shellQuote(ctx.remoteBin)); err != nil {
					return fmt.Errorf("failed to swap remote binary: %w", err)
				}
				if _, err := sudoRun(client, "systemctl restart dialtone-robot.service"); err != nil {
					return fmt.Errorf("failed to restart dialtone-robot.service after binary swap: %w", err)
				}
				return nil
			}
			if _, err := ssh_plugin.RunSSHCommand(client, "mv "+shellQuote(ctx.remoteBinTmp)+" "+shellQuote(ctx.remoteBin)); err != nil {
				return fmt.Errorf("failed to swap remote binary: %w", err)
			}
			return nil
		}},
		{name: "verify deployment health", run: func() error { return verifyDeployment(client, ctx.opts) }},
	}

	if ctx.opts.Relay {
		steps = append(steps, deployStep{name: "configure local cloudflare relay service", run: func() error {
			hostname := strings.TrimSpace(os.Getenv("DIALTONE_DOMAIN"))
			if hostname == "" {
				hostname = strings.TrimSpace(os.Getenv("DIALTONE_HOSTNAME"))
			}
			if hostname == "" {
				hostname = "drone-1"
			}
			return setupLocalCloudflareProxyService(hostname, ctx.opts.Host)
		}})
	}

	if ctx.opts.SmokeTest {
		steps = append(steps, deployStep{name: "post-deployment UI smoke test", run: RunPostDeployUIValidation})
	}

	if err := runDeploySteps(steps); err != nil {
		return err
	}
	logs.Info("[DEPLOY] Deployment complete")
	return nil
}
