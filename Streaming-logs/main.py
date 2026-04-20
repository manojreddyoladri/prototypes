from datetime import datetime
import os
from threading import Thread
import time
from uuid import uuid4
from faker.factory import logger
from flask import Flask, Response, redirect, render_template
from faker import Faker

DATASETS_LOGS = "./data"

fake = Faker()
app = Flask(__name__)

def mock_deployment(deployment_id: str):
    filepath = os.path.join(DATASETS_LOGS, f"{deployment_id}.log")
    logger.info("initiatiing deployment {}", deployment_id)
    logger.info("pushing logs to {}", filepath)
    with open(filepath, "a", encoding="utf-8") as fp:
        for _ in range(100000):
            fp.write(f"{datetime.now().isoformat()}: {fake.text(max_nb_chars=100)}\n")
            fp.flush()
            time.sleep(0.5)

@app.route("/", methods=["GET"])
def index_handler():
    deployments = [
        x.split(".")[0] for x in os.listdir(DATASETS_LOGS) if x.endswith(".log")
    ]
    return render_template("index.html", deployments=deployments)

@app.route("/deployments/<deployment_id>", methods=["GET"])
def deployment_handler(deployment_id: str):
    return render_template("deployment.html", deployment_id=deployment_id)

@app.route("/deployments", methods=["POST"])
def create_deployment_handler():
    deployment_id = uuid4().hex
    thread = Thread(target=mock_deployment, args=(deployment_id,))
    thread.start()

    return redirect(f"/deployments/{deployment_id}", 301)

def log_tailer(deployment_id: str):
    filepath = os.path.join(DATASETS_LOGS, f"{deployment_id}.log")
    with open(filepath, "r", encoding="utf-8") as fp:
        while True:
            line = fp.readline()
            if not line:
                time.sleep(0.1)
                continue
            yield f"data: {line.strip()}\n\n"

@app.route("/logs/<deployment_id>")
def stream_logs_handler(deployment_id: str):
    logs_stream = log_tailer(deployment_id)
    return Response(logs_stream, mimetype="text/event-stream")

if __name__ == "__main__":
    app.run(debug=True)