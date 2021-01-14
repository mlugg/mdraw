#!/usr/bin/env python3

import asyncio
import websockets

socks = []

async def test(ws, path):
    global socks
    socks += [ws]
    try:
        while True:
            cmd = await ws.recv()
            for s in socks:
                if s is not ws:
                    await s.send(cmd)
    except websockets.exceptions.ConnectionClosed:
        socks = [s for s in socks if s is not ws]

asyncio.get_event_loop().run_until_complete(websockets.serve(test, "localhost", 8081))
asyncio.get_event_loop().run_forever()
