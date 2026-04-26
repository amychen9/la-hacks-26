# Schej.it API

API docs (available when the server is running): http://localhost:3002/swagger/index.html

## Debug

- Install mongodb
- Install `air`, a package that facilitates live reload for Go apps
  - `go install github.com/cosmtrek/air@latest`
- To run the server, simply run `air` in the root directory of the server

## Make a backup of the mongodb database

- Run `mongodump --host="localhost:27017" --db=schej-it` to make a backup
- Run `mongorestore --uri mongodb://localhost:27017 ./dump --drop` to restore

## Fetch.ai / Agentverse integration

The backend now exposes Fetch-friendly routes that can be used when registering an agent on Agentverse:

- `GET /api/fetch/discovery` - agent metadata + capabilities for discovery.
- `GET /api/fetch/health` - liveness endpoint.
- `POST /api/fetch/chat` - Chat-Protocol-style endpoint backed by the scheduler recommendation engine.

Example request:

```json
{
  "message": "Find the best slot for this group",
  "context": {
    "sessionId": "YOUR_EVENT_ID",
    "meetingType": "study",
    "locationHint": "UCLA",
    "durationMinutes": 60,
    "timezone": "America/Los_Angeles",
    "currentLocation": {
      "latitude": 34.0689,
      "longitude": -118.4452
    }
  }
}
```
