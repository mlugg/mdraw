const canvas = document.getElementById("drawCanvas");
const cursorCanvas = document.getElementById("cursorCanvas");

var ctx = makeDrawContext(canvas, cursorCanvas, enableDraw, disableDraw);

// Input {{{

// Drawing {{{

function canvasPtrDown(e) {
	if (ctx.curMode != DrawMode.NONE) return;
	const p = ptrPressure(e);
	startDraw(ctx, p, ctx.x, ctx.y, e.button == 0 ? DrawMode.DRAW : DrawMode.ERASE);
	doToolOverlay(ctx, p);
}

// When the mouse is moved, run the handler for the current mode, and
// update the cursor
function canvasPtrMove(e) {
	const p = ptrPressure(e);
	const events = typeof e.getCoalescedEvents === "function" ? e.getCoalescedEvents() : [e];
	for (const ev of events) {
		doMove(ctx, p, ev.offsetX, ev.offsetY);
	}
	doToolOverlay(ctx, p);
}

function canvasPtrUp(e) {
	const p = ptrPressure(e);
	endDraw(ctx);
	doToolOverlay(ctx, p);
}

function lostPointer(e) {
	endDraw(ctx);
	clearToolOverlay(ctx);
}


function ptrPressure(e) {
	if (e.pressure == 0.5) return 1; // Device does not support pressure
	return e.pressure;
}

function enableDraw(ctx) {
	canvas.addEventListener("pointerdown", canvasPtrDown);
	canvas.addEventListener("pointermove", canvasPtrMove);
	canvas.addEventListener("pointerup", canvasPtrUp);
	canvas.addEventListener("lostpointercapture", lostPointer);
	canvas.addEventListener("pointerleave", lostPointer);
}

function disableDraw(ctx) {
	ctx.drawCtx.clearRect(0,0, ctx.width, ctx.height);
	canvas.removeEventListener("pointerdown", canvasPtrDown);
	canvas.removeEventListener("pointermove", canvasPtrMove);
	canvas.removeEventListener("pointerup", canvasPtrUp);
	canvas.removeEventListener("lostpointercapture", lostPointer);
	canvas.removeEventListener("pointerleave", lostPointer);
}

// }}}

document.getElementById("drawSize").addEventListener("change", e => ctx.drawSize = e.target.value);
document.getElementById("eraseSize").addEventListener("change", e => ctx.eraseSize = e.target.value);

document.getElementById("eraserBox").addEventListener("change", e => ctx.forceMode = e.target.checked ? DrawMode.ERASE : DrawMode.NONE);

document.getElementById("clear").addEventListener("click", e => clear(ctx));

document.getElementById("penColor").addEventListener("change", e => setColor(ctx, e.target.value));

document.getElementById("undo").addEventListener("click", e => undo(ctx));

// Don't open the context menu in the canvas (allows for rmb erase)
canvas.addEventListener("contextmenu", e => e.preventDefault());

document.addEventListener("keydown", e => {
	if (e.ctrlKey && e.key == 'z') {
		undo(ctx);
	}
});

// }}}
