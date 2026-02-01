# Plugin: plugin

The `plugin` command is used to manage Dialtone plugins.

## Usage
`./dialtone.sh plugin <command> [options]`

## Commands
- **add <plugin-name>** (or `create`): Scaffolds a new plugin in `src/plugins/<plugin-name>`.
- **install <plugin-name>**: Installs dependencies for the specified plugin.
- **build <plugin-name>**: Builds the specified plugin binary.
- **test <plugin-name>**: Runs tests for the specified plugin.
- **help**: Shows the help message.

## Extension
This CLI can be extended by modifying `src/plugins/plugin/cli/plugin.go`.
