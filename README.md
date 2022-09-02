# kolide-pipeline-notifier

Send notifications when Kolide Pipeline outputs change

## Features

* Support for Kolide Differential Logs output via the Log Pipeline
* Rich Slack notifications
* Google Cloud Storage
* CLI and HTTP server modes
* Duplicate event suppression

## Usage

Compile:

```shell
go build .
```

Setup your local credentials:

```shell
gcloud auth application-default login
```

Inspect output without sending notifications:

```shell
kolide-pipeline-notifier \
  --bucket=your-kolide-logs \
  --prefix=kolide/results/threat_hunting \
  --max-age=8h
```

To send notifications, add your Slack `--webhook-url`

## Webserver mode

This will run a web server, that will scan the bucket every time `/refreshz` is hit, as well as send notifications:

```shell
kolide-pipeline-notifier \
  --bucket=your-kolide-logs \
  --prefix=kolide/results/threat_hunting \
  --webhook-url=https://hooks.slack.com/services/oaeuSAOENTU/OESUNTOEU \
  --serve
```

This allows the kolide-pipeline-notifier to be run in environments that assume an HTTP frontend, such as Google Cloud Run. You can then use a scheduler service to hit `/refreshz` as often as you want to poll for results.

## Environment Variables

For your deployment, you may find it more useful to use environment variables than arguments. The `kolide-pipeline-notifier` supports a handful of them:

* `PORT`
* `BUCKET_NAME`
* `BUCKET_PREFIX`
* `WEBHOOK_URL`

## Google Cloud Run

Using `ko`, it is easy to build `kolide-pipeline-notifier` to your local repo and deploy it straight into production:

```shell
export KO_DOCKER_REPO="gcr.io/<your project>/pipeline-notifier"

gcloud run deploy pipeline-notifier \
  --image="$(ko publish .)" \
  --args=-serve \
  --region us-central1 \
  --project "<your project>"
```
