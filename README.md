# Downloader

Simple tool to download the same file from different sources. Also you could download multiple files from multiple sources simultaneously.

## Build
`make build`

## Usage
```
./bin/downloader -h

Usage:
        downloader [flags]

Flags:
        -h, --help              Help message
        -w, --with-sync         Download with sync means using file sync after successful download. Default: false.
        -c, --chunk-size        Chunk size in bytes. Default: 5MB. Min 512KB, Max 30MB.
        -f, --failed-chunks     Max     count of failed chunks to retry. Default: 10.
        -r, --retries           Max retries for every failed download file chunk. Default: 3.
        -s, --sources           Source file urls. Shoud be passed with "," as separator without space. Or every new source with  "-s" flag.

Example:
        downloader -s https://example.com/file1,https://example.com/file2 - for downloading two files from example.com
        downloader -s https://example1.com/file1 -s https://example2.com/file1 - for downloading one file from different sources
```

## Example
```bash
./bin/downloader -s https://mirror.math.princeton.edu/pub/ubuntu-iso/23.04/ubuntu-23.04-live-server-amd64.iso -s https://forksystems.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-live-server-amd64.iso -s https://southfront.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-live-server-amd64.iso -s https://southfront.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-netboot-amd64.tar.gz -s https://mirror.math.princeton.edu/pub/ubuntu-iso/23.04/ubuntu-23.04-netboot-amd64.tar.gz


2023/06/06 15:42:48 Starting download file ubuntu-23.04-netboot-amd64.tar.gz; sources:
 [https://southfront.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-netboot-amd64.tar.gz https://mirror.math.princeton.edu/pub/ubuntu-iso/23.04/ubuntu-23.04-netboot-amd64.tar.gz]

2023/06/06 15:42:48 Starting download file ubuntu-23.04-live-server-amd64.iso; sources:
 [https://mirror.math.princeton.edu/pub/ubuntu-iso/23.04/ubuntu-23.04-live-server-amd64.iso https://forksystems.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-live-server-amd64.iso https://southfront.mm.fcix.net/ubuntu-releases/23.04/ubuntu-23.04-live-server-amd64.iso]

Downloading ubuntu-23.04-netboot-amd64.tar.gz 148.22 MB / 148.22 MB [===============================================================================>] 100 %
Downloading ubuntu-23.04-live-server-amd64.iso 2.64 GB / 2.64 GB [==================================================================================>] 100 %

shasum -a 256 ubuntu-23.04-live-server-amd64.iso 
c7cda48494a6d7d9665964388a3fc9c824b3bef0c9ea3818a1be982bc80d346b  ubuntu-23.04-live-server-amd64.iso

./bin/downloader -s https://forksystems.mm.fcix.net/ubuntu-releases/23.04/SHA256SUMS
cat SHA256SUMS 
a8cd6ccff865e17dd136658f6388480c9a5bc57274b29f7d5bd0ed855a9281a5 *ubuntu-23.04-desktop-amd64.iso
c7cda48494a6d7d9665964388a3fc9c824b3bef0c9ea3818a1be982bc80d346b *ubuntu-23.04-live-server-amd64.iso
203dbe36c3718300b3f1da6ac9c43e0b47ef6446e961274e69f242223219e69b *ubuntu-23.04-netboot-amd64.tar.gz
```