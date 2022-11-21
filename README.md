# spy

A program that collects information about how often a program and files are used.
The intended use for this is to show the most often used programs / files and
the associated package.

# Requirements / Dependencies
- A C compiler
- The `proc` directory usually at `/proc`
- scdoc (for manpage)

# Issues
- It uses more cpu than I would like, but that's mainly due to the files that
  are read in `/proc`. Using something like `lsof` every second or so will
  provide a similar usage. You could use the `-i` flag to change it.
- `spy -s` takes a long time because of listing all the packages and listing
  all the files for the each of those packages.

# Installing

```
git clone https://github.com/ShaqeelAhmad/spy
cd spy
make
sudo make install
```

# Usage

Note:
* `spy -s` assumes spy_list_packages and spy_list_package_files are in PATH
  and are executable.
* `spy -s` might be really slow, so output it to another file
  (e.g `spy -s > spy_data`) and then use the file for other operations.

```
spy                  # Start collecting information
spy -m               # Collect mapped files as well. Search for "map_files" in proc(5)
spy -f ~/.spy.db     # Store collected data in ~/.spy.db
spy -i 60            # Collect data every 60 seconds
spy -s | sort -n     # Display packages usage and sort it by frequency it's used
spy -s | sort -k2 -n # Display packages usage and sort it by the last time it's used
spy -s | grep '^0'   # show the packages that are not used at all
```


# License

GPLv3
