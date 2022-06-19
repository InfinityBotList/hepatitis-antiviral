# Listens to events from the migrator and prompts the user when needed for input.
# This is a daemon process that runs in the foreground

from fastapi import FastAPI
import pydantic
import tqdm
import uvicorn
import sys

import inspect
# store builtin print
old_print = print
def new_print(*args, **kwargs):
    # if tqdm.tqdm.write raises error, use builtin print
    try:
        tqdm.tqdm.write(*args, **kwargs)
    except:
        old_print(*args, ** kwargs)
# globaly replace print with new_print
inspect.builtins.print = new_print

app = FastAPI()

@app.get("/")
def read_root():
    return {}

class Notify(pydantic.BaseModel):
    loglevel: str
    message: str

class Progress(pydantic.BaseModel):
    total: int
    done: int
    col: str

class Message():
    def __init__(self):
        self.msg = []

    async def __aiter__(self):
        return self
    
    async def __anext__(self):
        i = 0
        # Wait for user input at i
        while True:
            try:
                print(self.msg[i])
                break
            except:
                continue
        yield self.msg[i]
        i += 1

app.state.msg = Message()

@app.post("/notify")
async def notify(notify: Notify):
    if notify.loglevel == "error":
        # Red color
        print("\033[1;31m" + notify.message + "\033[0m")
    elif notify.loglevel == "warning":
        # Yellow color
        print("\033[1;33m" + notify.message + "\033[0m")
    elif notify.loglevel == "info":
        # Blue color
        print("\033[1;34m" + notify.message + "\033[0m")
    elif notify.loglevel == "debug":
        # Grey color
        print("\033[1;30m" + notify.message + "\033[0m")
    else:
        print(notify.message)
    return {}

@app.post("/progress")
async def progressbar_(p: Progress):
    bar: tqdm.tqdm = app.state.pbar

    # Close old bar
    if p.done == 0:
        app.state.pbar = tqdm.tqdm(None, desc=p.col, position=0, file=sys.stdout, total=p.total)
    bar.update(p.done - app.state.done)

    app.state.done = p.done
    app.state.total = p.total

@app.on_event("startup")
async def startup():
    app.state.done = 0
    app.state.total = 0
    app.state.pbar = tqdm.tqdm(None, desc="Progress", position=0, file=sys.stdout)

uvicorn.run(app, port=3939, access_log=False, log_level="critical")