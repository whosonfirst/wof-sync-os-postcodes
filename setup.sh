#!/bin/bash
set -eo pipefail

MOUNT_DIR="/mnt/wof"

sudo mkdir -p "$MOUNT_DIR"
sudo mount -t tmpfs -o size=90%,nr_inodes=0 wof "$MOUNT_DIR"
sudo chown $(whoami):$(whoami) "$MOUNT_DIR"

# Add stuff for building, and other useful utils
sudo apt install -y build-essential git golang tmux unzip jq

cd "$MOUNT_DIR"

git clone --depth 1 https://github.com/whosonfirst-data/whosonfirst-data-admin-gb
git clone https://github.com/whosonfirst-data/whosonfirst-data-postalcode-gb
# Disable GC because it really hurts the commit performance if it kicks in, and this checkout is not long lived
cd whosonfirst-data-postalcode-gb
git config gc.auto 0
cd ..

# Grab the latest release
DOWNLOAD_URL=$(curl -sL wofcurl -sL "https://api.github.com/repos/whosonfirst/wof-sync-os-postcodes/releases/latest" | jq -r '.assets[].browser_download_url' | grep linux_x86_64)
curl -sL "${DOWNLOAD_URL}" -o wof-sync-os-postcodes
chmod +x wof-sync-os-postcodes

git clone https://github.com/whosonfirst/go-whosonfirst-pip-v2
cd go-whosonfirst-pip-v2
make tools
cp bin/wof-pip-server ..
cd ..