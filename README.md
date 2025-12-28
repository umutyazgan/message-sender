# message-service

This is a simple web service that sends messages to a predetermined webhook URL every 2 minutes.

## Requirements

 - go
 - mongodb
 - docker / docker-compose
 - redis

## Build and Run the Service

First run using docker-compose:

```
docker compose up --build
```

This should build and run the app, db and the redis services. In the first run, db will be populated with some dummy data. `--build` flag is not necessary for later runs.

Take the containers down with:

```
docker compose down
```

Or, if you would like to wipe the data volume as well:

```
docker compose down -v
```
