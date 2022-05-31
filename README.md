# spy

a program that collects data about the processes that run on your system.
The intended use for this is to show the most often / commonly used package and
to find the packages that are not used by anything.

# Requirements / Dependencies
- A go 1.16 or later compiler*
- The `proc` directory usually at `/proc`
- scdoc (for manpage)

\* Checkout the go1.13 branch to compile using older go versions (1.13 to
1.15). This is done because some things are deprecated or not recommended in
newer versions of Go, specifically the `ioutil` package whose functionalities
are now provided by `os` and `io` pacakge. Go version 1.13 and newer should
work with the branch.

# Issues
- It uses more cpu than I would like, but that's mainly due to the files that
  are read in `/proc`. Using something like `lsof` every second or so will
  provide a similar usage. You could set `interval` in the config file to
  match your preferences.
- Updating the database (`spy update`) takes a long time because of listing
  all the packages and listing all the files for the packages.

# Installing

```
git clone https://github.com/ShaqeelAhmad/spy
# For the 1.13 branch: git clone -b go1.13 https://github.com/ShaqeelAhmad/spy
cd spy
make
sudo make install
```

# Usage
Note:
* Terminals give SIGHUP to a process running in it when the terminal is
  quitting and spy might not quit because SIGHUP causes reloading config file.
  To actually kill it use SIGTERM or SIGINT.
* `spy update` assumes spy-list_packages and spy-list_package_files are in PATH
  and are executable.

```
spy collect           # Start collecting information
killall -SIGHUP spy   # Reload config file
spy update            # Update the database
spy show | sort -n    # List the usage for packages and sort it
spy show | grep '^0'  # show the packages that are not used at all
```

# License

GPLv3
