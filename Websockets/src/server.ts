import 'reflect-metadata';
import path from 'path';
import express from 'express';
import http from 'http';
import { Server } from 'socket.io';
import { AppDataSource } from './data-source';
import { User } from './entities/User';

const app = express();
const server = http.createServer(app);

const io = new Server(server, {
  cors: {
    origin: '*',
  },
});

// Simple in-memory mapping of socket.id -> username for this prototype
const socketUserMap = new Map<string, string>();

async function markUserOnline(username: string) {
  const repo = AppDataSource.getRepository(User);
  let user = await repo.findOne({ where: { username } });
  if (!user) {
    user = repo.create({ username, online: true });
  } else {
    user.online = true;
  }
  await repo.save(user);
}

async function markUserOffline(username: string) {
  const repo = AppDataSource.getRepository(User);
  const user = await repo.findOne({ where: { username } });
  if (user) {
    user.online = false;
    await repo.save(user);
  }
}

io.on('connection', (socket) => {
  console.log('Socket connected', socket.id);

  // Expect client to send their username as part of an "register" event
  socket.on('register', async (username: string) => {
    console.log(`User registered for socket ${socket.id}: ${username}`);
    socketUserMap.set(socket.id, username);
    try {
      await markUserOnline(username);
    } catch (err) {
      console.error('Error marking user online', err);
    }
  });

  // Custom disconnecting handler
  socket.on('disconnect', async (reason) => {
    console.log('Socket disconnected', socket.id, 'reason:', reason);
    const username = socketUserMap.get(socket.id);
    if (username) {
      socketUserMap.delete(socket.id);
      try {
        await markUserOffline(username);
        console.log(`Marked ${username} offline in DB`);
      } catch (err) {
        console.error('Error marking user offline', err);
      }
    }
  });
});

app.use(express.static(path.join(__dirname, '..', 'public')));

const PORT = process.env.PORT || 4000;

AppDataSource.initialize()
  .then(() => {
    server.listen(PORT, () => {
      console.log(`Server listening on http://localhost:${PORT}`);
    });
  })
  .catch((err) => {
    console.error('Error during Data Source initialization', err);
  });

