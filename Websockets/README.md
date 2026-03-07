# Websockets

A WebSocket prototype demonstrating how to track user online/offline status in real time using Socket.IO and a SQLite database.

## Description

This prototype shows how WebSockets are used to determine whether a user is online or offline. When a client connects and registers with a username, the server marks the user as online in the database. When the client disconnects, the server updates the user status to offline. This provides a basic understanding of WebSocket concepts and real-time connection handling.

## Motivation

Create a WebSocket prototype to build a foundational understanding of how WebSockets work, how connect and disconnect events flow between client and server, and how server-side logic can react to these events (e.g., persisting user status to a database).

## Features

- **Connect event**: Client establishes a WebSocket connection and emits a `register` event with a username. The server receives it and marks the user as **online** in the database.
- **Disconnect event**: When the client disconnects (browser closed, network lost, or manual disconnect), the server detects it and marks the user as **offline** in the database.
- Simple web UI with Connect/Disconnect buttons and status display.
- SQLite database storing user online/offline state.

## Tech Stack

- **Node.js** – Runtime
- **Express** – HTTP server and static file serving
- **Socket.IO** – WebSocket library for real-time bidirectional communication
- **TypeORM** – ORM for database operations
- **SQLite** – Embedded database

## Prerequisites

- Node.js (v18 or higher recommended)
- npm

## Installation

1. Clone or navigate to the project directory:

   ```bash
   cd Websockets
   ```

2. Install dependencies:

   ```bash
   npm install
   ```

3. Start the development server:

   ```bash
   npm run dev
   ```

   The server will start at `http://localhost:4000` (or the port set by the `PORT` environment variable).

## Usage

### 1. Start the server

```bash
npm run dev
```

### 2. Open the application

Open your browser and go to:

```
http://localhost:4000
```

### 3. Connect

- Enter a username in the input field (default: `alice`).
- Click **Connect** or wait for auto-connect on page load.
- The status will show **connected**, and the server will mark the user as **online** in the database.

### 4. Disconnect

- Click **Disconnect** to close the WebSocket connection.
- The status will show **disconnected**.
- The server will update the user as **offline** in the database and log the event in the terminal.

### 5. Verify in the database

To inspect user status in SQLite:

```bash
sqlite3 db.sqlite "SELECT * FROM user;"
```

## Project Structure

```
Websockets/
├── public/
│   ├── index.html    # Web UI
│   └── app.js        # Client-side Socket.IO logic
├── src/
│   ├── server.ts     # Express + Socket.IO server
│   ├── data-source.ts
│   └── entities/
│       └── User.ts   # User entity
├── db.sqlite         # SQLite database (created on first run)
├── package.json
└── tsconfig.json
```

## Events

| Event       | Direction   | Description                                                  |
| ----------- | ----------- | ------------------------------------------------------------ |
| `connection`| Server      | Fired when a client connects.                                |
| `register`  | Client → Server | Client sends username to register and mark user online. |
| `disconnect`| Server      | Fired when a client disconnects; server marks user offline.  |

## Notes

This is a simple prototype focused on connect and disconnect emit events. It does not include authentication, reconnection strategies, or production-ready error handling. It is intended for learning WebSocket basics.
