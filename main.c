#define _DEFAULT_SOURCE
#define _POSIX_C_SOURCE 200809L

#include <assert.h>
#include <dirent.h>
#include <errno.h>
#include <limits.h>
#include <signal.h>
#include <stdarg.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#define ENVPATH_CAP 32
#define HASHTABLE_CAP 256

struct HashtableEntry {
	struct HashtableEntry *next;
	unsigned long hash;
	char *key;
	uint64_t value;
	uint64_t time; /* unix seconds */
};

static struct HashtableEntry *hashtable[HASHTABLE_CAP];

/* $PATH split by ':' into multiple paths */
static char *envPaths[ENVPATH_CAP];
static size_t envPaths_len = 0;

/* Variables that may be changed by the user */
static char *procDir = "/proc";
static char *logFile = "/var/log/spy.db";
static int collectMappedFiles = 0;
static unsigned int sleepInterval = 1;
static int debug = 0;

/*
 * global variable to use with getdelim which will reallocate memory as
 * necessary. There isn't a reason to free the memory since we are constantly
 * using it.
 */
static char *lineptr;
static size_t lineptr_len;

static char *ignoredMapFilePrefixes[] = {
	"anon_inode",
	"/memfd",
	"/root",
	"/home",
	"/proc",
	"/dev",
	"/tmp",
	"/var",
};

static void
debugLog(char *fmt, ...)
{
	if (!debug) return;

	va_list ap;
	va_start(ap, fmt);
	vfprintf(stderr, fmt, ap);
	va_end(ap);
}

static void *
ecalloc(size_t nmemb, size_t size)
{
	void *p = calloc(nmemb, size);
	if (p == NULL) {
		perror("calloc");
		exit(1);
	}
	return p;
}

void *
erealloc(void *ptr, size_t size)
{
	void *p = realloc(ptr, size);
	if (p == NULL) {
		perror("realloc");
		exit(1);
	}

	return p;
}

static char *
estrdup(char *s)
{
	s = strdup(s);
	if (s == NULL) {
		perror("strudp");
		exit(1);
	}
	return s;
}

static void
initEnvPaths(void)
{
	char *s = getenv("PATH");
	if (s == NULL || *s == '\0') {
		fprintf(stderr, "PATH is not defined\n");
		exit(1);
	}
	s = estrdup(s);

	/* for freeing later */
	envPaths[0] = s;
	for (;*s;) {
		char *end = s;
		for (;*end && *end != ':'; end++);

		assert(envPaths_len < ENVPATH_CAP &&
				"Something is wrong with you for having so many paths in $PATH.");

		envPaths[envPaths_len] = s;
		envPaths_len++;
		if (*end) {
			*end = '\0';
			end++;
		}
		s = end;
	}
}

static unsigned long
hash(char *str)
{
	unsigned long hash = 5381;
	int c;

	while ((c = *str++))
		hash = ((hash << 5) + hash) + c;

	return hash;
}

static int
hashtable_Get(char *key, uint64_t *valuep, uint64_t *timep)
{
	unsigned long h = hash(key);
	unsigned long n = h % HASHTABLE_CAP;
	struct HashtableEntry *ht = hashtable[n];

	for (; ht != NULL; ht = ht->next) {
		if (ht->hash == h && strcmp(key, ht->key) == 0) {
			if (timep) *timep = ht->time;
			if (valuep) *valuep = ht->value;
			return 0;
		}
	}

	return -1;
}

static uint64_t
hashtable_GetValue(char *key)
{
	uint64_t value = 0;
	hashtable_Get(key, &value, NULL);

	return value;
}


static void
hashtable_Set(char *key, uint64_t value, uint64_t time)
{
	unsigned long h = hash(key);
	unsigned long n = h % HASHTABLE_CAP;
	struct HashtableEntry *ht = hashtable[n];
	struct HashtableEntry *prev = NULL;
	while (ht != NULL) {
		if (ht->hash == h && strcmp(key, ht->key) == 0) {
			ht->time = time;
			ht->value = value;
			return;
		}
		prev = ht;
		ht = ht->next;
	}
	ht        = ecalloc(1, sizeof(struct HashtableEntry));
	ht->hash  = h;
	ht->key   = estrdup(key);
	ht->time  = time;
	ht->value = value;
	if (prev != NULL) {
		prev->next = ht;
	} else {
		hashtable[n] = ht;
	}
}

static void
printHashtable(FILE *f)
{
	for (int i = 0; i < HASHTABLE_CAP; i++) {
		struct HashtableEntry *ht = hashtable[i];
		while (ht != NULL) {
			fprintf(f, "%lu\t%lu\t", ht->value, ht->time);
			char *s = ht->key;
			/* escape '\n', '\t' and '\\' */
			for (; *s; s++) {
				switch (*s) {
				case '\t':
					fputs("\\t", f);
					break;
				case '\n':
					fputs("\\n", f);
					break;
				case '\\':
					fputs("\\\\", f);
					break;
				default:
					fputc(*s, f);
					break;
				}
			}
			fputc('\n', f);

			ht = ht->next;
		}
	}
}


static int
hasPrefix(char *p, char *s)
{
	for (;*p && *s; p++, s++)
		if (*p != *s)
			return 0;

	return *p == '\0';
}

static void
collect(void)
{
	struct timespec tp = {
		.tv_sec = 0,
	};
	if (clock_gettime(CLOCK_REALTIME , &tp) < 0) {
		perror("clock_gettime");
	}
	uint64_t time = tp.tv_sec;

	struct dirent *d = NULL;
	DIR *dir = opendir(procDir);
	if (dir == NULL) {
		perror(procDir);
		exit(1);
	}
	char path[PATH_MAX];

	debugLog("Listing proc: %s\n", procDir);
loop:
	while ((d = readdir(dir)) != NULL) {
		if (strcmp(d->d_name, ".") == 0 || strcmp(d->d_name, "..") == 0
				|| d->d_type != DT_DIR) {
			continue;
		}
		char *s = d->d_name;

		/* quit if directory name has non-digits */
		for (; *s; s++)
			if (*s < '0' || *s > '9')
				goto loop;

		{
			snprintf(path, PATH_MAX, "%s/%s/cmdline", procDir, d->d_name);
			FILE *f = fopen(path, "r");
			if (f == NULL) {
				continue;
			}

			if (getdelim(&lineptr, &lineptr_len, '\0', f) < 0 || lineptr[0] == '\0') {
				fclose(f);
				continue;
			}
			if (lineptr[0] != '/') {
				for (size_t i = 0; i < envPaths_len; i++) {
					snprintf(path, PATH_MAX, "%s/%s", envPaths[i], lineptr);
					if (access(path, X_OK) < 0) {
						continue;
					}
					uint64_t n = hashtable_GetValue(path);
					hashtable_Set(path, n+1, time);
					break;
				}
			} else {
				uint64_t n = hashtable_GetValue(lineptr);
				hashtable_Set(lineptr, n+1, time);
			}

			fclose(f);
		}

		if (collectMappedFiles) {
			snprintf(path, PATH_MAX, "%s/%s/map_files/", procDir, d->d_name);
			DIR *dir = opendir(path);
			if (dir == NULL) {
				continue;
			}
			struct dirent *d;
			char filepath[PATH_MAX];
			char buf[PATH_MAX];
mapFileLoop:
			while ((d = readdir(dir)) != NULL) {
				if (strcmp(d->d_name, ".") == 0
						|| strcmp(d->d_name, "..") == 0) {
					continue;
				}
				strcpy(filepath, path);
				strcat(filepath, d->d_name);
				int n = readlink(filepath, buf, PATH_MAX);
				if (n < 0) {
					continue;
				}
				buf[n] = 0;

				int len = sizeof(ignoredMapFilePrefixes) /
					sizeof(ignoredMapFilePrefixes[0]);

				for (int i = 0; i < len; i++)
					if (hasPrefix(ignoredMapFilePrefixes[i], buf))
							goto mapFileLoop;

				hashtable_Set(buf, hashtable_GetValue(buf)+1, time);
			}
			closedir(dir);
		}
	}

	closedir(dir);
}

static int
readFileFields(FILE *f, uint64_t *value, uint64_t *time)
{
	char *endptr = NULL;
	int n = getdelim(&lineptr, &lineptr_len, '\t', f);
	if (n < 0) {
		return -1;
	}
	lineptr[n-1] = '\0';
	*value = strtoul(lineptr, &endptr, 10);
	if (endptr == lineptr || *endptr != '\0') {
		return -1;
	}

	n = getdelim(&lineptr, &lineptr_len, '\t', f);
	if (n < 0) {
		return -1;
	}
	lineptr[n-1] = '\0';
	*time = strtoul(lineptr, &endptr, 10);
	if (endptr == lineptr || *endptr != '\0') {
		return -1;
	}

	n = getline(&lineptr, &lineptr_len, f);
	if (n < 0) {
		return -1;
	}
	n--;
	lineptr[n] = '\0';

	/* unescape '\n', '\t' and '\\' */
	int i = 0;
	for (int j = 0; j < n && lineptr[j]; i++, j++) {
		if (lineptr[j] == '\\') {
			j++;
			switch (lineptr[j]) {
				case 'n':
					lineptr[j] = '\n';
					break;
				case 't':
					lineptr[j] = '\t';
					break;
			}
		}
		lineptr[i] = lineptr[j];
	}
	lineptr[i] = 0;

	return 0;
}

static void
readLogFile(char *file)
{
	debugLog("Reading logfile %s\n", logFile);

	FILE *f = fopen(file, "r");
	if (f == NULL)
		return;

	uint64_t time, value;
	for (;;) {
		if (readFileFields(f, &value, &time) < 0) {
			break;
		}

		value += hashtable_GetValue(lineptr);
		hashtable_Set(lineptr, value, time);
	}

	fclose(f);
}
struct Package {
	char *pkg;
	char **files;
	size_t files_len;
};

struct PackageList {
	struct Package *pkgs;
	size_t pkgs_len;
};

static struct PackageList
getPkgFiles(void)
{
	FILE *f = popen("spy_list_packages", "r");
	if (f == NULL) {
		perror("spy_list_packages");
		exit(1);
	}
	struct PackageList list = {0};
	int n;
	char *cmd = NULL;
	while ((n = getline(&lineptr, &lineptr_len, f)) > 0) {
		lineptr[n-1] = 0;
		list.pkgs_len++;
		list.pkgs = erealloc(list.pkgs, list.pkgs_len * sizeof(struct Package));
		list.pkgs[list.pkgs_len-1] = (struct Package){
			.files = NULL,
			.files_len = 0,
			.pkg = estrdup(lineptr),
		};
		hashtable_Set(lineptr, 0, 0);

		{
			struct Package *pkg = &list.pkgs[list.pkgs_len-1];
			char *s = "spy_list_package_files ";
			cmd = erealloc(cmd, sizeof(char) * (strlen(s) + n + 1));
			strcpy(cmd, s);
			strcat(cmd, lineptr);
			FILE *f = popen(cmd, "r");
			if (f == NULL) {
				perror(cmd);
				continue;
			}
			while ((n = getline(&lineptr, &lineptr_len, f)) > 0) {
				lineptr[n-1] = 0;
				pkg->files_len++;
				pkg->files = erealloc(pkg->files, pkg->files_len * sizeof(char*));
				pkg->files[pkg->files_len-1] = estrdup(lineptr);
			}
			pclose(f);
		}
	}

	free(cmd);

	pclose(f);
	return list;
}

int
comparFunc(const void *a, const void *b)
{
	char *const *s1 = a;
	char *const *s2 = b;
	return strcmp(*s1, *s2);
}

void
printData(void)
{
	FILE *f = fopen(logFile, "r");
	if (f == NULL) {
		perror(logFile);
		exit(1);
	}

	struct PackageList list = getPkgFiles();

	uint64_t time, value;
	for (;;) {
		if (readFileFields(f, &value, &time) < 0) {
			break;
		}

		for (size_t i = 0; i < list.pkgs_len; i++) {
			char **s = bsearch(&lineptr, list.pkgs[i].files,
					list.pkgs[i].files_len, sizeof(char*),
					comparFunc);
			if (s != NULL) {
				uint64_t v = 0, t = 0;
				hashtable_Get(list.pkgs[i].pkg, &v, &t);
				if (t > time)
					time = t;

				hashtable_Set(list.pkgs[i].pkg, v + value, time);
				break;
			}
		}
	}

	for (size_t i = 0; i < list.pkgs_len; i++) {
		for (size_t j = 0; j < list.pkgs[i].files_len; j++) {
			free(list.pkgs[i].files[j]);
		}
		free(list.pkgs[i].files);
		free(list.pkgs[i].pkg);
	}
	free(list.pkgs);

	fclose(f);

	printHashtable(stdout);
}

static void
usage(void)
{
	printf("usage: spy [-vsdm] [-f dataFile] [-i interval] [-p procDir]\n");
	printf("\t-v           show the spy version and exit.\n");
	printf("\t-s           show the collected data.\n");
	printf("\t-d           debug logging.\n");
	printf("\t-m           collect mapped files. Look for \"map_files\" in proc(5).\n");
	printf("\t-f dataFile  specify the file to write the collected data to.\n");
	printf("\t-i interval  the seconds to sleep between each data collection.\n");
	printf("\t-p procDir   the path to the proc filesystem. Look at proc(5).\n");
}


static sig_atomic_t running = 1;

static void
handleSignal(int sig)
{
	running = 0;
	debugLog("\nReceived signal %d %s.\n", sig,
			sig == SIGINT ? "SIGINT" : "SIGTERM");
}

int
main(int argc, char **argv)
{
	int showData = 0;
	int c;
	while ((c = getopt(argc, argv, "vsdmf:i:p:")) != -1) {
		switch (c) {
		case 'v':
			puts("spy: "SPY_VERSION);
			exit(0);
		case 's':
			showData = 1;
			break;
		case 'd':
			debug = 1;
			break;
		case 'f':
			logFile = optarg;
			break;
		case 'm':
			collectMappedFiles = 1;
			break;
		case 'p':
			procDir = optarg;
			break;
		case 'i':;
			char *endptr;
			unsigned int i = strtoul(optarg, &endptr, 10);
			if (*endptr != '\0' || i <= 0) {
				fprintf(stderr, "Error: option -i requires positive integer argument\n");
				exit(1);
			}
			sleepInterval = i;
			break;
		default:
			usage();
			exit(1);
		}
	}
	if (showData) {
		printData();
		return 0;
	}

	readLogFile(logFile);

	initEnvPaths();

	debugLog("Handling SIGINT and SIGTERM\n");
	struct sigaction sa;
	memset(&sa, 0, sizeof(sa));
	sa.sa_handler = handleSignal;
	if (sigaction(SIGINT,  &sa, NULL) < 0) {
		debugLog("sigaction: failed to handle SIGINT: %s\n", strerror(errno));
	}
	if (sigaction(SIGTERM, &sa, NULL) < 0) {
		debugLog("sigaction: failed to handle SIGTERM: %s\n", strerror(errno));
	}
	sa.sa_flags |= SA_RESTART;

	debugLog("Starting data collection loop\n");
	while (running) {
		debugLog("Collecting data\n");
		collect();

		debugLog("Writing to file %s\n", logFile);
		FILE *f = fopen(logFile, "w");
		if (f == NULL) {
			perror(logFile);
			fprintf(stderr, "exiting...\n");

			/* The collected data is useless if we can't write to a file */
			exit(1);
		}
		printHashtable(f);
		fclose(f);

		if (!running) break;

		debugLog("Sleeping for %d second(s)\n", sleepInterval);
		sleep(sleepInterval);
	}
	debugLog("\nCleaning up...\n");

	for (int i = 0; i < HASHTABLE_CAP; i++) {
		struct HashtableEntry *ht = hashtable[i], *prev = NULL;
		while (ht != NULL) {
			prev = ht;
			ht = ht->next;
			free(prev->key);
			free(prev);
		}
	}

	free(envPaths[0]);
	free(lineptr);

	return 0;
}
