package devcontainer

// GlobalOptions holds devcontainer CLI options shared between Up and Exec.
type GlobalOptions struct {
	WorkspaceFolder string            // --workspace-folder
	Config          string            // --config (path to devcontainer.json)
	OverrideConfig  string            // --override-config (override any devcontainer.json)
	DockerPath      string            // --docker-path
	RemoteEnv       map[string]string // --remote-env (applied to up and exec)
	Mounts          []string          // --mount (applied to up only)
	DotfilesRepo    string            // --dotfiles-repository
	DotfilesCommand string            // --dotfiles-install-command
	DotfilesPath    string            // --dotfiles-target-path
	SecretsFile     string            // --secrets-file
}

// baseArgs returns CLI arguments common to both Up and Exec.
func (g *GlobalOptions) baseArgs() []string {
	var args []string
	if g.WorkspaceFolder != "" {
		args = append(args, "--workspace-folder", g.WorkspaceFolder)
	}
	if g.OverrideConfig != "" {
		args = append(args, "--override-config", g.OverrideConfig)
	} else if g.Config != "" {
		args = append(args, "--config", g.Config)
	}
	if g.DockerPath != "" {
		args = append(args, "--docker-path", g.DockerPath)
	}
	for k, v := range g.RemoteEnv {
		args = append(args, "--remote-env", k+"="+v)
	}
	return args
}

// upArgs returns CLI arguments specific to devcontainer up.
func (g *GlobalOptions) upArgs() []string {
	var args []string
	for _, m := range g.Mounts {
		args = append(args, "--mount", m)
	}
	if g.DotfilesRepo != "" {
		args = append(args, "--dotfiles-repository", g.DotfilesRepo)
	}
	if g.DotfilesCommand != "" {
		args = append(args, "--dotfiles-install-command", g.DotfilesCommand)
	}
	if g.DotfilesPath != "" {
		args = append(args, "--dotfiles-target-path", g.DotfilesPath)
	}
	if g.SecretsFile != "" {
		args = append(args, "--secrets-file", g.SecretsFile)
	}
	return args
}
