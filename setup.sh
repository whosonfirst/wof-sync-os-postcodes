#!/bin/bash
set -eo pipefail

WORKING_DIR="/mnt/wof"
PERSISTENT_DIR="./wof-cmd"

# Set 2KB per inode
sudo mkfs.ext4 -i 2048 -F /dev/nvme0n1
sudo mkdir -p "$WORKING_DIR"
sudo mount /dev/nvme0n1 "$WORKING_DIR"
sudo chown $(whoami):$(whoami) "$WORKING_DIR"

# Install Golang PPA so we have latest
sudo add-apt-repository -y ppa:longsleep/golang-backports
sudo apt -y update

# Add stuff for building, and other useful utils
sudo apt install -y build-essential git golang tmux unzip jq libsqlite3-dev libicu-dev

cd "$WORKING_DIR"

git clone https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
# Disable GC because it really hurts the commit performance if it kicks in, and this checkout is not long lived
cd whosonfirst-data-postalcode-gb
git config gc.auto 0
cd ..

git clone https://github.com/whosonfirst-data/whosonfirst-data-admin-gb

# Grab the latest release
DOWNLOAD_URL=$(curl -sL "https://api.github.com/repos/whosonfirst/wof-sync-os-postcodes/releases/latest" | jq -r '.assets[].browser_download_url' | grep linux_x86_64)
curl -sL "${DOWNLOAD_URL}" -o "$PERSISTENT_DIR/wof-sync-os-postcodes"
chmod +x "$PERSISTENT_DIR/wof-sync-os-postcodes"

cd "$PERSISTENT_DIR"
