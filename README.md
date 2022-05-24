# spy

a program that collects data about the processes that run on your system.
The intended use for this is to show the most often / commonly used package and
to find the packages that are not used by anything.

# Requirements
- A go 1.16 or later compiler*
- The `proc` directory usually at `/proc`

\* Checkout the go1.13 branch to compile using older go versions. This is done
because some things are deprecated or not recommended in newer versions of Go,
specifically the `ioutil` package whose functionalities are now provided by `os` and
`io` pacakge. Go version 1.13 and newer should work with the branch.

# Issues
- It uses more cpu than I would like, but that's mainly due to the files that
  are read in `/proc`. Using something like `lsof` every second or so will
  provide a similar usage. You could set `interval` in the config file to
  match your preferences.
- Updating the database (`spy update`) takes a long time*  because of listing
  all the packages and listing all the files for the packages.

\* It takes me about 29.257 seconds on a cold cache and 3.993 seconds on a warm
cache. I have about 311 packages and 304514 files listed. The long time
is probably due to using a 10~ year old computer without a SSD.

# Installing

```
git clone https://github.com/ShaqeelAhmad/spy
cd spy
make
sudo make install
```

# Usage

```
spy collect # Start collecting information
```

Note that terminals give SIGHUP to a process running in it when the terminal is
quitting and spy might not quit. To actually kill it use SIGTERM or SIGINT.
```
killall -SIGHUP spy # Reload config file
```

`spy update` assumes spy-list_packages and spy-list_package_files are in PATH
and are executable.
```
spy update # Update the database of packages with the collected information
```

```
spy show # List the usage for packages
```

# License

GPLv3
