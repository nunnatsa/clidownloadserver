# clidownloadserver

This is a simple file server. For security, it support non-hierarchy directory (flat directory structure, no sub
directories).

The files are stored in the "files" directory as gzip files and served with the "Content-Encoding: gzip" HTTP
response header (if the "accept-encoding: gzip" HTTP request header was sent).

We assumes the existance of the metadata/files.json file on build time. This is handled in build/build.sh

To build/build.sh downloads the latest virtctl files from the latest KubeVirt release in github, compress them and generates the files.json file.

TODO:
* [ ] Auto compute the latest KubeVirt version
* [ ] Handle logging, use better logging library
* [ ] TLS & http2
