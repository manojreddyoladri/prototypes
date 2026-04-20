# Server Sent Events Prototype

A minimal Flask app that **streams fake deployment logs** to the browser using **[Server-Sent Events (SSE)](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events)**. The UI lists past “deployments” and live-updates new log lines as they are written—no database, no heavy styling—so you can focus on how SSE works and why it shows up everywhere in modern APIs (including LLM streaming).

---

## Description

This prototype showcases **one long-lived HTTP response** with `Content-Type: text/event-stream`, and a tiny **EventSource** client that prepends each incoming line to a list. It is intentionally small: one directory of `.log` files, a background thread that simulates work, and a few routes.

## Motivation

I built this to learn **how Server-Sent Events are implemented end-to-end**—from the HTTP response shape (`data: …\n\n`) to the browser’s `EventSource` API—and to have a concrete reference when comparing **SSE vs WebSockets** for real product features.

## Features

- **Home page (`/`)** – Lists past deployments by scanning the `data/` folder for `*.log` files; “New Deployment” submits a form to start another run.
- **POST `/deployments`** – Creates a new deployment ID (UUID), starts a **background thread** that writes log lines to a file, then **redirects** to the deployment page.
- **GET `/deployments/<deployment_id>`** – Renders `deployment.html` with the live log UI.
- **GET `/logs/<deployment_id>`** – **SSE stream**: reads the log file line-by-line and emits [`text/event-stream`](https://html.spec.whatwg.org/multipage/server-sent-events.html) events the browser can consume with `EventSource`.
- **Client** – Uses `EventSource` and **`onmessage`**; each event’s text is turned into a `<li>` and **prepended** to the list so newest lines appear at the top.

## Tech stack

| Layer | Choice |
|--------|--------|
| Language | [Python](https://www.python.org/) 3 |
| Web framework | [Flask](https://flask.palletsprojects.com/) |
| Fake log text | [Faker](https://faker.readthedocs.io/) |
| Streaming | SSE (`text/event-stream`), [EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource) (browser API) |

---

## Installation

### Prerequisites

- Python 3.10+ recommended  
- `pip` (or `uv` / your preferred installer)

### Steps

1. **Enter the project directory**

   ```bash
   cd Streaming-logs
   ```

2. **Create a virtual environment (recommended)**

   ```bash
   python3 -m venv .venv
   ```

3. **Install dependencies**

   ```bash
   .venv/bin/pip install -r requirements.txt
   ```

4. **Ensure the log directory exists** (the app writes under `./data`)

   ```bash
   mkdir -p data
   ```

---

## Usage

**Start the server** (use the venv’s Python so `flask` / `faker` resolve):

```bash
.venv/bin/python main.py
```

Open **http://127.0.0.1:5000** (Flask’s default).

- Click **New Deployment** to POST to `/deployments` and open the new deployment page.
- Watch log lines appear in the list as the background thread appends to the `.log` file and the SSE endpoint streams them.

**Quick checks with `curl`:**

```bash
# List is HTML; open in browser. To trigger a deployment:
curl -X POST -L http://127.0.0.1:5000/deployments

# Stream raw SSE (replace DEPLOYMENT_ID):
curl -N http://127.0.0.1:5000/logs/DEPLOYMENT_ID
```

---

## Understanding the prototype

This README is meant to support **two discussions at once**: **(1) how to “box” a prototype** so you actually finish it and learn one thing, and **(2) how Server-Sent Events work** in real systems—including why **almost every LLM app** you use over HTTP is, in spirit, the same **streaming response** idea.

---

### Part 1 — Boxing the prototype (scope, storage, UI)

**Boxing** means drawing a hard boundary: *one idea in the center*, everything else made deliberately boring.

- **No database** — No tables, no migrations, no SQL. There is only a folder, `data/`. Each “deployment” is **one file**: `<deployment_id>.log`. Each line is a **timestamp** plus **fake text** (from Faker). To show “all deployments that ever happened,” the server does `os.listdir`, keeps paths that end in `.log`, splits off the id (the part before `.log`), and passes that list into the template. That is the entire persistence story.
- **No CSS rabbit hole** — The UI is bare on purpose: a **New Deployment** button inside a `<form>`, a **Deployments** heading, a `<ul>` of `<li>` items, each an `<a>` to `/deployments/<id>`. You see past runs and you can start a new one. No design system, no wasted time on styling—the goal is to **watch streaming behavior**, not pixels.
- **No real cloud** — Nothing provisions EC2. **`mock_deployment`** only **pretends** a long job: open the log file, loop a large number of times, each iteration append one line (`datetime` + fake text up to ~100 characters), **`flush`**, **`sleep(0.5)`**, repeat. That **half-second sleep** is why you see **roughly one new line every half second** in the UI—enough to *feel* like a live deployment without integrating AWS or CI.

So the box is: **files + background thread + minimal HTML**, and the only “interesting” piece is **how lines reach the browser** (SSE).

---

### Part 2 — What happens when you click “New Deployment”

1. **Home (`GET /`)** — Renders `index.html` with the deployment list and the form. That’s it.

2. **Submit the form (`POST /deployments`)** — The handler generates a **UUID** as `deployment_id`, starts a **daemon thread** that runs **`mock_deployment(deployment_id)`**, and **redirects** (301) to **`GET /deployments/<deployment_id>`**. You land on the deployment page **while** the thread keeps writing to disk—so the “deployment” is genuinely in progress in the background.

3. **`mock_deployment`** — For that id, it writes to `data/<deployment_id>.log`. The loop is intentionally dumb: append, flush, sleep. No orchestration, no queues—**minimum code** to simulate work.

4. **Deployment page (`GET /deployments/<deployment_id>`)** — Renders **`deployment.html`**: show the id, a **Home** link, an empty **`<ul id="logs">`**, and a short **`<script>`**. The list is where live lines will appear.

---

### Part 3 — The browser: `EventSource`, prepend, and why it feels “live”

The script is where the client connects to the stream:

- You construct **`new EventSource(\`/logs/${deploymentId}\`)`**. That URL must be an **SSE endpoint**—the same pattern works for **browser → server** or **server → server**; the idea is still “one HTTP response that keeps producing data.”
- On **`onmessage`**, you read **`event.data`**, create an **`<li>`**, set its text, and **`prepend`** it to `#logs` so **new lines appear at the top**. That **prepend** (instead of append) is a small UX choice: it reads like a **live tail** in reverse—each half-second line “pops in” at the top, which feels more dynamic than scrolling a growing list downward.

So the **client never busy-polls** your REST API in a `setInterval`. The **server-side tailer** does the waiting; the browser just consumes events on **one long request**.

---

### Part 4 — The server: `/logs/<deployment_id>`, HTTP streaming, and the SSE format

**HTTP intuition:** A normal response is “here is the full body, done.” A **streaming** response says: “the body is **not** finished yet—expect more.” In practice you often see **chunked** bodies or incremental writes; the connection stays useful while bytes keep arriving. For browsers, the special case you care about here is **`Content-Type: text/event-stream`** — the [Server-Sent Events](https://html.spec.whatwg.org/multipage/server-sent-events.html) content type. In Flask you return a **`Response`** with `mimetype="text/event-stream"`. In another language (e.g. Go), you’d set the header yourself—the **shape of the program** is still: **one route**, **one handler**, **one response object** you write to over time. You are **not** passing a WebSocket handle through half your codebase; you’re just **emitting bytes** on a regular HTTP response.

**Wire format:** SSE is intentionally minimal. Each event often looks like:

```text
data: <payload>\n\n
```

**Two consecutive newlines** (`\n\n`) end **one** event. So the server opens the log file, **reads line by line**; if there is no new line yet, **sleep** and try again (like `tail -f`); when a line exists, emit **`data: <that line>\\n\\n`**. That’s the whole protocol surface you need for this demo—**no WebSocket framing**, no custom binary protocol.

---

### Part 5 — Why this pattern dominates LLM APIs (and similar UIs)

When you use an **LLM over HTTP**, the common experience is: **one request** (“answer this prompt / this chat id”), then **many chunks** of response body arriving over time—**tokens** streaming in the **same request**. That is structurally the same as this demo: **one request**, **many `data:` lines**, client renders as they arrive. So understanding SSE / streaming HTTP is not academic—it is the **same mental model** as “the model is still generating.”

---

### Part 6 — SSE vs WebSockets: choose by product, not only by arrows on a diagram

Avoid reducing the decision to **“bidirectional = expensive, one-way = cheap.”** That misses **what you’re building** and **how it should feel**.

- **SSE fits** when the story is **mostly server → client**: log tailing, job progress, **LLM token streams**, “step 3 of 15 done” updates. You expose **one URL** that **streams**; the client uses **`EventSource`** (or reads a stream). **CI/CD UX** is a good intuition: user kicks off a pipeline; the UI shows *checking… checking… done* per stage. You *could* model that with WebSockets, but often the **simpler** design is: **one HTTP call** that **streams progress events** as SSE (`data:` lines + blank line between events)—same idea as this repo.
- **WebSockets fit** when you need a **persistent, low-latency, bidirectional** channel—**chat**, **multiplayer**, **collaborative editing**, **live market data** where the client also sends a lot on the same connection.

**WebSockets are not “bad.”** They’re the wrong default when you only need **server → client streaming over HTTP**—which is exactly what **LLM streaming** and **log UIs** usually are.

---

### Part 7 — Takeaway

**Boxing** = one feature, fake the rest with a directory and a thread. **SSE** = **`text/event-stream`** + **`data: …`** lines terminated by **blank line** (`\n\n`) + **`EventSource`** on the client. **Product choice** = match transport to **user experience** (streaming status vs real-time chat), not only to a **bidirectional vs unidirectional** sketch on a whiteboard.

---

## Project layout

```
Streaming-logs/
├── main.py              # Flask app, mock deployment thread, SSE generator
├── requirements.txt
├── data/                # One .log file per deployment (gitignored if you add it)
└── templates/
    ├── index.html       # List deployments + new deployment form
    └── deployment.html  # EventSource + log list
```

---

## Notes

- Ensure **`data/`** exists before first run (see Installation).
- Run with **`.venv/bin/python main.py`** if your global Python doesn’t have the dependencies installed.
- This is a **learning prototype**, not production-hardened: no auth, no backpressure tuning, no graceful shutdown of the writer thread.
