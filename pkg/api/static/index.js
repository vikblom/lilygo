const canvas = document.getElementById('drawing-board');
const toolbar = document.getElementById('toolbar');
const ctx = canvas.getContext('2d');

const canvasOffsetX = canvas.offsetLeft;
const canvasOffsetY = canvas.offsetTop;

canvas.width = Math.min(540, screen.width-10);
canvas.height = Math.min(960, screen.height-10-toolbar.offsetHeight); // Minus toolbar height.
// canvas.width = window.innerWidth - canvasOffsetX;
// canvas.height = window.innerHeight - canvasOffsetY;

let isPainting = false;
let lineWidth = 5;
let startX;
let startY;
ctx.strokeStyle = getGrayValue(0);

toolbar.addEventListener('click', e => {
    if (e.target.id === 'clear') {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    }
    if (e.target.id === 'submit') {
      var dataURL = canvas.toDataURL();
      console.log(dataURL);
      fetch('/image', {
        method: 'POST',
        headers: {'Content-Type':'application/x-www-form-urlencoded'},
        body: dataURL
      }).then((e) => {
        if (e.status === 200) {
          ctx.clearRect(0, 0, canvas.width, canvas.height);
        } else {
          window.alert("Something went wrong!");
        }
      }).catch((err) => {
        console.log(err);
        window.alert("Something went wrong!");
      })
    }
});

toolbar.addEventListener('input', e => {
    if (e.target.matches('input[type=range]')) {
        updateRangeBubble(e.target);
    }
});

toolbar.addEventListener('change', e => {
    if(e.target.id === 'lineWidth') {
        lineWidth = Number(e.target.value);
        updateRangeBubble(e.target);
    }
    if(e.target.id === 'color') {
        ctx.strokeStyle = getGrayValue(Number(e.target.value));
        updateRangeBubble(e.target);
    }
});

function getGrayValue(level) {
    const gray = Math.round((level / 15) * 255);
    return `rgb(${gray}, ${gray}, ${gray})`;
}

function updateRangeBubble(range) {
    const wrapper = range.closest('.range-wrapper');
    if (!wrapper) return;
    const valueLabel = wrapper.querySelector('.range-value');
    if (!valueLabel) return;

    const min = Number(range.min);
    const max = Number(range.max);
    const value = Number(range.value);
    const percent = max === min ? 0 : (value - min) / (max - min) * 100;

    wrapper.style.setProperty('--range-percent', `${percent}%`);

    if (range.id === 'color') {
        valueLabel.textContent = value === 0 ? 'Black' : value === 15 ? 'White' : `Gray ${value}`;
    } else {
        valueLabel.textContent = value;
    }
}

window.addEventListener('DOMContentLoaded', () => {
    const lineWidthRange = document.getElementById('lineWidth');
    const colorRange = document.getElementById('color');
    if (lineWidthRange) updateRangeBubble(lineWidthRange);
    if (colorRange) updateRangeBubble(colorRange);
});

const draw = (e) => {
    if(!isPainting) {
        return;
    }

    ctx.lineWidth = lineWidth;
    ctx.lineCap = 'round';

    ctx.lineTo(e.clientX - canvasOffsetX, e.clientY - canvasOffsetY);
    ctx.stroke();
}

canvas.addEventListener('mousedown', (e) => {
    isPainting = true;
    startX = e.clientX;
    startY = e.clientY;
});

canvas.addEventListener('mouseup', e => {
    isPainting = false;
    ctx.stroke();
    ctx.beginPath();
});

canvas.addEventListener('mousemove', draw);

const viewport = window.visualViewport;
var offsetX;
var offsetY;

// Touch code.
// https://leimao.github.io/blog/HTML-Canvas-Touch-Drawing

function startup() {
  canvas.addEventListener('touchstart', handleStart);
  canvas.addEventListener('touchend', handleEnd);
  canvas.addEventListener('touchcancel', handleCancel);
  canvas.addEventListener('touchmove', handleMove);

  console.log(canvas.width);
  console.log(canvas.height);
  console.log(screen.width);
  console.log(screen.height);
}

document.addEventListener("DOMContentLoaded", startup);

const ongoingTouches = [];

function handleStart(evt) {
  evt.preventDefault();
  const touches = evt.changedTouches;
  offsetX = canvas.getBoundingClientRect().left;
  offsetY = canvas.getBoundingClientRect().top;
  console.log(offsetX, offsetY)
  for (let i = 0; i < touches.length; i++) {
    ongoingTouches.push(copyTouch(touches[i]));
  }
}

function handleMove(evt) {
  evt.preventDefault();
  const touches = evt.changedTouches;
  for (let i = 0; i < touches.length; i++) {
    //const color = document.getElementById('selColor').value;
    const idx = ongoingTouchIndexById(touches[i].identifier);
    if (idx >= 0) {
      ctx.beginPath();
      ctx.moveTo(ongoingTouches[idx].clientX - offsetX, ongoingTouches[idx].clientY - offsetY);
      ctx.lineTo(touches[i].clientX - offsetX, touches[i].clientY - offsetY);
      ctx.lineWidth = lineWidth;
      // ctx.strokeStyle = color;
      ctx.lineJoin = "round";
      ctx.closePath();
      ctx.stroke();
      ongoingTouches.splice(idx, 1, copyTouch(touches[i]));  // swap in the new touch record
    }
  }
}

function handleEnd(evt) {
  evt.preventDefault();
  const touches = evt.changedTouches;
  for (let i = 0; i < touches.length; i++) {
    // const color = document.getElementById('selColor').value;
    let idx = ongoingTouchIndexById(touches[i].identifier);
    if (idx >= 0) {
      ctx.lineWidth = lineWidth;
      // ctx.fillStyle = color;
      ongoingTouches.splice(idx, 1);  // remove it; we're done
    }
  }
}

function handleCancel(evt) {
  evt.preventDefault();
  const touches = evt.changedTouches;
  for (let i = 0; i < touches.length; i++) {
    let idx = ongoingTouchIndexById(touches[i].identifier);
    ongoingTouches.splice(idx, 1);  // remove it; we're done
  }
}

function copyTouch({ identifier, clientX, clientY }) {
  return { identifier, clientX, clientY };
}

function ongoingTouchIndexById(idToFind) {
  for (let i = 0; i < ongoingTouches.length; i++) {
    const id = ongoingTouches[i].identifier;
    if (id === idToFind) {
      return i;
    }
  }
  return -1;    // not found
}

function clearArea() {
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, ctx.canvas.width, ctx.canvas.height);
}
