# http-response-collector

Retrieves HTTP responses and headers from specified endpoints and publishes the collected data to Google Cloud Pub/Sub for further processing.

## References

- [http-response-collector](https://github.com/UnitVectorY-Labs/http-response-collector) - Retrieves HTTP responses and headers from specified endpoints and publishes the collected data to Google Cloud Pub/Sub for further processing.
- [http-response-collector-tofu](https://github.com/UnitVectorY-Labs/http-response-collector-tofu) - OpenTofu module for deploying a http-response-collector to GCP

## Features

- Processes requests from Pub/Sub for fetching specified URLs.
- Retrieves HTTP responses from specified URLs.
- Extracts response headers and body content.
- Publishes structured response data to Google Cloud Pub/Sub as JSON.

## Configuration

The application requires the following environment variables:

| Variable               | Description                                      |
|------------------------|--------------------------------------------------|
| `GOOGLE_CLOUD_PROJECT` | The GCP project ID where Pub/Sub is hosted.      |
| `RESPONSE_PUBSUB`      | The Pub/Sub topic name for publishing responses. |

## Request Format

The following JSON format is used to request a URL to be fetched:

```json
{"url":"https://example.com"}
```

## Response Format

The following show examples of the payloads that are published to Pub/Sub.

A successful request whose body is JSON will include the `responseJson` payload:

```json
{
  "url": "https://example.com/content.json",
  "headers": "{\"Cache-Control\":\"max-age=3600, public, s-maxage=7200, stale-if-error=43200, stale-while-revalidate=3600, immutable\",\"Content-Type\":\"application/json\",\"Date\":\"Tue, 04 Feb 2025 23:37:31 GMT\"}",
  "responseJson": "{\"message\":\"Hello, World!\"}",
  "responseTime": 366,
  "requestTime": "2025-02-04T23:37:31.64365949Z",
  "statusCode": 200
}
```

A successful request whose body is not JSON will include the:

```json
{
  "url": "https://example.com/text",
  "headers": "{\"Content-Length\":\"22\",\"Content-Type\":\"text/plain\",\"Date\":\"Tue, 04 Feb 2025 23:48:27 GMT\"}",
  "responseBody": "Body Content Goes Here",
  "responseTime": 111,
  "requestTime": "2025-02-04T23:48:27.307539426Z",
  "statusCode": 200
}
```

A failed request will include the `error` payload:

```json
{
  "url": "https://fail.example.com",
  "error": "Error fetching URL",
  "requestTime": "2025-02-05T01:27:41.915539558Z",
}
```
