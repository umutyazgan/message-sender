#!/bin/sh
set -e

# If your JSON is an array of documents, add --jsonArray
mongoimport \
  --host localhost \
  --db insiderone \
  --collection messages \
  --file /docker-entrypoint-initdb.d/data.json \
  --jsonArray
