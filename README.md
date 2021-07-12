# clidownloadserver

This is a simple file server. For security, it contains a fixed list of downloadable files, and serves only them.

The files are stored in the "files" directory as gzip files

The server supports three ways to download the files:
1. If the Accept-Encoding request header contains `gzip`, the server will serve the files "as is" (compressed) and will
   add the "Content-Encoding: gzip" HTTP response header.
2. If the Accept-Encoding is missing or not contains `gzip`, the server will serve the uncompressed file.
3. if the request is for the file with the `.gz` extension, the server will serve the uncompressed file without the
   `"Content-Encoding: gzip"` HTTP response header.

Building with `go build .` assumes the existences of the metadata/files.json file on build time. This is handled in
build/build.sh

To build/build.sh downloads the latest virtctl files from the latest KubeVirt release in github, compress them and
generates the files.json file, and creates a docker image with the server, already populated with the compressed files.

TODO:
* [ ] Auto compute the latest KubeVirt version
* [ ] Handle logging, use better logging library
* [ ] TLS & http2
