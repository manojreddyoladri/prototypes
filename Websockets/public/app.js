let socket = null;
const status = document.getElementById('status');

function connect() {
  if (socket) return;
  status.textContent = 'connecting...';
  socket = io();
  socket.on('connect', function () {
    status.textContent = 'connected';
    socket.emit('register', document.getElementById('username').value);
  });
  socket.on('disconnect', function () {
    status.textContent = 'disconnected';
    socket = null;
  });
}

document.getElementById('connect').onclick = connect;
document.getElementById('disconnect').onclick = function () {
  if (socket) {
    socket.disconnect();
    socket = null;
  }
};

connect();
