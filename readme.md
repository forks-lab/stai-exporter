# Note

This is a modified version of chia-exporter for usage with STAI. It was created by refactoring of main branch (cd39c32).

# STAI Exporter

STAI Exporter is an application that is intended to run alongside a STAI installation and exports prometheus style metrics based on data available from STAI RPCs. Where possible, all data is received as events from websocket subscriptions. Some data that is not available as a metrics event is also fetched as well, but usually in response to an event that was already received that indicates the data may have changed (with the goal to only make as many RPC requests as necessary to get accurate metric data).

**_This project is actively under development and relies on data that may not yet be available in a stable release of STAI Blockchain. Dev builds of STAI may contain bugs or other issues that are not present in tagged releases. We do not recommend that you run pre-release/dev versions of STAI Blockchain on mission critical systems._**

<!-- not available for STAI

## Installation

Download the correct executable file from the release page and run. If you are on debian/ubuntu, you can install using the apt repo, documented below.


### Apt Repo Installation

#### Set up the repository

1. Update the `apt` package index and install packages to allow apt to use a repository over HTTPS:

```shell
sudo apt-get update

sudo apt-get install ca-certificates curl gnupg
```

2. Add STAI's official GPG Key:

```shell
curl -sL https://repo.chia.net/FD39E6D3.pubkey.asc | sudo gpg --dearmor -o /usr/share/keyrings/chia.gpg
```

3. Use the following command to set up the stable repository.

```shell 
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/chia.gpg] https://repo.chia.net/chia-exporter/debian/ stable main" | sudo tee /etc/apt/sources.list.d/chia-exporter.list > /dev/null
```

#### Install STAI Exporter

1. Update the apt package index and install the latest version of STAI Exporter

```shell
sudo apt-get update

sudo apt-get install chia-exporter
```

-->

## Usage

First, install [chia-blockchain](https://github.com/STATION-I/stai-blockchain). STAI exporter expects to be run on the same machine as the chia blockchain installation, and will use either the default chia config (`~/.stai/mainnet/`) or else the config located at `STAI_ROOT`, if the environment variable is set.

`stai-exporter serve` will start the metrics exporter on the default port of `9914`. Metrics will be available at `<hostname>:9914/metrics`.

### Configuration

Configuration options can be passed using command line flags, environment variables, or a configuration file, except for `--config`, which is a CLI flag only. For a complete listing of options, run `stai-exporter --help`.

To set a config value as an environment variable, prefix the name with `STAI_EXPORTER_`, convert all letters to uppercase, and replace any dashes with underscores (`metrics-port` becomes `STAI_EXPORTER_METRICS_PORT`).

To use a config file, create a new yaml file and place any configuration options you want to specify in the file. The config file will be loaded by default from `~/.stai-exporter`, but the location can be overridden with the `--config` flag.

```yaml
metrics-port: 9914
```

## Country Data

When running alongside the crawler, the exporter can optionally export metrics indicating how many peers have been discovered in each country, based on IP address. To enable this functionality, you will need to download the MaxMind GeoLite2 Country database and provide the path to the MaxMind database to the exporter application. The path can be provided with a command line flag `--maxmind-db-path /path/to/GeoLite2-Country.mmdb`, an entry in the config yaml file `maxmind-db-path: /path/to/GeoLite2-Country.mmdb`, or an environment variable `CHIA_EXPORTER_MAXMIND_DB_PATH=/path/to/GeoLite2-Country.mmdb`. To gain access to the MaxMind DB, you can [register here](https://www.maxmind.com/en/geolite2/signup).
