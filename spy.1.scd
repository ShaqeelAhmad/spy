spy(1)

# NAME

spy - Collect information on active processes.

# SYNOPSIS

_spy_ [options] <command>

# OPTIONS
*-h*
	Show a short help message

*-c* <config>
	Specify a config file

*-d*
	Enable logging information.

# COMMAND

*collect*
	Collect information about the currently active processes and store them in
	the data directory. There may be some reading issues as the proc directory
	may change and remove some directories when processes have been killed or
	have finished running. These are logged in debug mode.

*update*
	Update the database information about the packages. This might take a while
	depending on the list_packages and list_package_files command and how many
	packages you have.

*show*
	Show the package usage frequency. Remember to update the database first with
	*update* to get the newest data.

*help*
	Shows a short help message.

# ENVIRONMENT VARIABLES

_XDG\_DATA\_HOME_
	The directory to store the database. Defaults to HOME/.local/share if unset.

_XDG\_CONFIG\_HOME_
	The directory to search for the config file. Defaults to HOME/.config if unset.

# CONFIG FILE
Must be in $XDG_CONFIG_HOME/spy/config

*interval*
	A negative number means only collect once and quit. Any other number
	corresponds to the seconds that the program sleeps before collecting
	information again.

*procDir*
	Read *proc*(5).

*ignoredPrefix*
	The prefixes that are ignored. There are certain directories where you would
	not want to store information about them like the `/tmp` directory. Though
	this implementation only cares about the prefix, so giving any pattern that
	matches with the first parts of a path will be ignored.

*dbFile*
	The database file to write to. This should be an absolute path because
	they're not expanded (e.g ~ and $HOME are taken literally and a file with
	the name ~ or $HOME will be created)

Copy the example config file in EXAMPLE_CONFIG_PATH to
$XDG_CONFIG_HOME/spy/config. The file in EXAMPLE_CONFIG_PATH is not read by the
program and serves only as an example to the user.

# SIGNALS
*SIGHUP*
	Reload the config file

*SIGTERM*, *SIGQUIT*
	Quit

# HELPER SCRIPTS

*spy-list_packages*
	This script lists out the installed packages.

*spy-list_package_files*
	This scripts lists the files belonging to a package.

# SEE ALSO
*proc*(5)
