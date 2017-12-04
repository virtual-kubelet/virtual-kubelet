# HyperCLI [![Build Status](https://travis-ci.org/hyperhq/hypercli.svg?branch=master)](https://travis-ci.org/hyperhq/hypercli)

Go version of Hyper.sh client command line tools. 

## Install

### Quick and Easy (Recommended)

Grab the latest version for your system on the [Releases](https://github.com/hyperhq/hypercli/releases) page or build it by yourself as the [instruction](#how-to-build).

You can either run the binary directly or add somewhere in your $PATH.

## Getting Started

#### Before Getting Started

Before you can use Hyper.sh, be sure you've [created a free account with Hyper.sh](http://www.hyper.sh) and [generate your credentials on Hyper.sh](https://docs.hyper.sh/GettingStarted/generate_api_credential.html).

Once the installation and setup completes, enter `hyper config` in your terminal. The CLI will prompt to ask for your API credential:

![](https://trello-attachments.s3.amazonaws.com/56daae9b816ec930c8d98197/720x143/9fdd9a68694376d4ec62a3d93409e67c/upload_3_18_2016_at_6_11_19_PM.png)

The credential is stored in a local configuration file `$HOME/.hyper/config.json`. The configuration file is similar to Docker's, with an extra section `clouds`.

![](https://trello-attachments.s3.amazonaws.com/56daae9b816ec930c8d98197/635x160/c9caa016982d5884eb06578292c154bf/config.png)

Or you can use environmental vairables `HYPER_ACCESS` and `HYPER_SECRET` to pass the access key and secret key (CLI will search for these envs before loading the configuration file).

You only need to do that once for your machine. If you've done that, then you can continue.

[See the official docs](http://docs.hyper.sh/) for more detailed info on using Hyper.sh.

#### Actually Getting Started

The easiest way to get started is by digging around.

`$ hyper --help` for example usage and a list of commands

## How to build

```
$ mkdir $GOPATH/src/github.com/hyperhq/
$ cd $GOPATH/src/github.com/hyperhq/
$ git clone https://github.com/hyperhq/hypercli hypercli
$ cd hypercli
$ ./build.sh
```

## Contributing

Give us a pull request! File a bug!

