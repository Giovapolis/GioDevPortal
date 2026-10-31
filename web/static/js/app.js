(function () {
  function csrf() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  function initKanban() {
    if (typeof Sortable === "undefined") return;
    document.querySelectorAll("[data-kanban-column]").forEach(function (col) {
      if (col.dataset.sortableInit === "1") return;
      col.dataset.sortableInit = "1";
      new Sortable(col, {
        group: "kanban",
        animation: 150,
        ghostClass: "opacity-50",
        onEnd: function (evt) {
          var taskId = evt.item.dataset.taskId;
          var status = evt.to.dataset.status;
          var board = evt.item.closest("#kanban-board");
          if (!taskId || !status || !board) return;
          var projectId = board.dataset.projectId;
          var siblings = Array.prototype.slice.call(evt.to.querySelectorAll("[data-task-id]"));
          var order = siblings.indexOf(evt.item);
          htmx.ajax("POST", "/admin/projects/" + projectId + "/kanban/tasks/" + taskId + "/move", {
            target: "#kanban-board",
            swap: "outerHTML",
            headers: { "X-CSRF-Token": csrf() },
            values: { status: status, order: order }
          });
        }
      });
    });
  }

  function initCircuitBackground() {
    var canvas = document.getElementById("circuit-background");
    if (!canvas || typeof canvas.getContext !== "function") return;

    var ctx = canvas.getContext("2d");
    if (!ctx) return;

    var reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
    var GRID = 44;
    var MAX_DPR = 2;
    var width = 0;
    var height = 0;
    var dpr = 1;
    var traces = [];
    var branches = [];
    var time = 0;
    var running = false;
    var raf = 0;
    var staticRaf = 0;
    var lastFrame = 0;
    var resizeTimer = 0;
    var pointer = { x: 0, y: 0, active: false };
    var glowSprite = makeGlowSprite();

    function makeGlowSprite() {
      var size = 64;
      var sprite = document.createElement("canvas");
      sprite.width = size;
      sprite.height = size;
      var sctx = sprite.getContext("2d");
      var gradient = sctx.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
      gradient.addColorStop(0, "rgba(255, 240, 255, 0.95)");
      gradient.addColorStop(0.2, "rgba(232, 121, 249, 0.8)");
      gradient.addColorStop(0.55, "rgba(168, 85, 247, 0.32)");
      gradient.addColorStop(1, "rgba(168, 85, 247, 0)");
      sctx.fillStyle = gradient;
      sctx.fillRect(0, 0, size, size);
      return sprite;
    }

    function clamp(value, min, max) {
      return Math.max(min, Math.min(max, value));
    }

    function mulberry32(seed) {
      return function () {
        seed |= 0;
        seed = (seed + 0x6d2b79f5) | 0;
        var t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
        t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
      };
    }

    function makeTrace(points) {
      var segments = [];
      var total = 0;
      for (var i = 1; i < points.length; i++) {
        var a = points[i - 1];
        var b = points[i];
        var dx = b.x - a.x;
        var dy = b.y - a.y;
        var length = Math.sqrt(dx * dx + dy * dy);
        if (length < 1) continue;
        segments.push({ x1: a.x, y1: a.y, x2: b.x, y2: b.y, length: length });
        total += length;
      }
      return { points: points, segments: segments, total: total };
    }

    function buildTraces() {
      traces = [];
      branches = [];

      var seed = ((width | 0) * 73856093) ^ ((height | 0) * 19349663) ^ 0x9e3779b9;
      var random = mulberry32(seed);
      var cols = Math.max(2, Math.floor(width / GRID));
      var rows = Math.max(2, Math.floor(height / GRID));
      var count = width < 640 ? 14 : 22;

      for (var i = 0; i < count; i++) {
        var side = Math.floor(random() * 4);
        var x = 0;
        var y = 0;
        var horizontal = true;
        var firstStep = 1;
        var gtY = (1 + Math.floor(random() * Math.max(1, rows - 1))) * GRID;
        var gtX = (1 + Math.floor(random() * Math.max(1, cols - 1))) * GRID;

        if (side === 0) {
          x = 0;
          y = Math.min(gtY, height);
          horizontal = true;
          firstStep = 1;
        } else if (side === 1) {
          x = Math.min(cols * GRID, width);
          y = Math.min(gtY, height);
          horizontal = true;
          firstStep = -1;
        } else if (side === 2) {
          x = Math.min(gtX, width);
          y = 0;
          horizontal = false;
          firstStep = 1;
        } else {
          x = Math.min(gtX, width);
          y = Math.min(rows * GRID, height);
          horizontal = false;
          firstStep = -1;
        }

        var points = [{ x: x, y: y }];
        var segmentCount = 3 + Math.floor(random() * 3);

        for (var s = 0; s < segmentCount; s++) {
          var direction = s === 0 ? firstStep : random() < 0.18 ? -1 : 1;
          var distance = (1 + Math.floor(random() * 5)) * GRID * direction;
          var nextX = x;
          var nextY = y;

          if (horizontal) {
            nextX = clamp(x + distance, 0, width);
            if (nextX === x) {
              nextX = clamp(x - distance, 0, width);
            }
          } else {
            nextY = clamp(y + distance, 0, height);
            if (nextY === y) {
              nextY = clamp(y - distance, 0, height);
            }
          }

          if (nextX !== x || nextY !== y) {
            x = nextX;
            y = nextY;
            points.push({ x: x, y: y });
          }
          horizontal = !horizontal;
        }

        if (points.length < 2) continue;

        var trace = makeTrace(points);
        if (trace.total < GRID) continue;
        trace.speed = 24 + random() * 42;
        trace.offset = random() * trace.total;
        trace.brightness = 0.55 + random() * 0.45;
        trace.near = 0;
        traces.push(trace);

        if (points.length > 2 && random() < 0.55) {
          var branchPoint = points[1 + Math.floor(random() * (points.length - 2))];
          var branchLength = (1 + Math.floor(random() * 4)) * GRID * (random() < 0.2 ? -1 : 1);
          var branchX = branchPoint.x;
          var branchY = branchPoint.y;

          if (random() > 0.5) {
            branchX = clamp(branchPoint.x + branchLength, 0, width);
          } else {
            branchY = clamp(branchPoint.y + branchLength, 0, height);
          }

          if (branchX !== branchPoint.x || branchY !== branchPoint.y) {
            branches.push({
              x1: branchPoint.x,
              y1: branchPoint.y,
              x2: branchX,
              y2: branchY
            });
          }
        }
      }
    }

    function pointAt(trace, distance) {
      if (trace.total <= 0) return trace.points[0];
      var remaining = ((distance % trace.total) + trace.total) % trace.total;
      for (var i = 0; i < trace.segments.length; i++) {
        var segment = trace.segments[i];
        if (remaining <= segment.length) {
          var ratio = remaining / segment.length;
          return {
            x: segment.x1 + (segment.x2 - segment.x1) * ratio,
            y: segment.y1 + (segment.y2 - segment.y1) * ratio
          };
        }
        remaining -= segment.length;
      }
      return trace.points[trace.points.length - 1];
    }

    function proximityToSegment(px, py, x1, y1, x2, y2) {
      var vx = x2 - x1;
      var vy = y2 - y1;
      var lengthSquared = vx * vx + vy * vy;
      if (lengthSquared < 1) return 0;
      var ratio = ((px - x1) * vx + (py - y1) * vy) / lengthSquared;
      ratio = clamp(ratio, 0, 1);
      var dx = px - (x1 + vx * ratio);
      var dy = py - (y1 + vy * ratio);
      var distance = Math.sqrt(dx * dx + dy * dy);
      var radius = width < 640 ? 120 : 170;
      if (distance >= radius) return 0;
      var strength = 1 - distance / radius;
      return strength * strength * (3 - 2 * strength);
    }

    function proximityFor(trace) {
      if (!pointer.active) return 0;
      var best = 0;
      for (var i = 0; i < trace.segments.length; i++) {
        var segment = trace.segments[i];
        var near = proximityToSegment(pointer.x, pointer.y, segment.x1, segment.y1, segment.x2, segment.y2);
        if (near > best) best = near;
      }
      return best;
    }

    function drawBranches() {
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
      for (var i = 0; i < branches.length; i++) {
        var branch = branches[i];
        var near = pointer.active
          ? proximityToSegment(pointer.x, pointer.y, branch.x1, branch.y1, branch.x2, branch.y2)
          : 0;
        ctx.beginPath();
        ctx.moveTo(branch.x1, branch.y1);
        ctx.lineTo(branch.x2, branch.y2);
        ctx.strokeStyle = "rgba(168, 85, 247, " + (0.05 + near * 0.14) + ")";
        ctx.lineWidth = 0.7 + near * 0.6;
        ctx.stroke();
        ctx.beginPath();
        ctx.arc(branch.x2, branch.y2, 0.9 + near * 0.6, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(217, 70, 239, " + (0.12 + near * 0.3) + ")";
        ctx.fill();
      }
    }

    function drawTraces() {
      ctx.lineCap = "round";
      ctx.lineJoin = "round";
      for (var i = 0; i < traces.length; i++) {
        var trace = traces[i];
        var near = proximityFor(trace);
        trace.near = near;

        ctx.beginPath();
        ctx.moveTo(trace.points[0].x, trace.points[0].y);
        for (var p = 1; p < trace.points.length; p++) {
          ctx.lineTo(trace.points[p].x, trace.points[p].y);
        }
        ctx.strokeStyle = "rgba(168, 85, 247, " + (0.06 + near * 0.22) + ")";
        ctx.lineWidth = 0.85 + near * 0.9;
        ctx.stroke();

        ctx.fillStyle = "rgba(217, 70, 239, " + (0.13 + near * 0.32) + ")";
        for (var n = 0; n < trace.points.length; n++) {
          ctx.beginPath();
          ctx.arc(trace.points[n].x, trace.points[n].y, 0.9 + near * 0.65, 0, Math.PI * 2);
          ctx.fill();
        }
      }
    }

    function drawPulses() {
      ctx.save();
      ctx.globalCompositeOperation = "lighter";
      for (var i = 0; i < traces.length; i++) {
        var trace = traces[i];
        var near = trace.near || 0;
        var distance = trace.offset + time * trace.speed * (1 + near * 1.6);
        var pulse = pointAt(trace, distance);
        var size = 16 + near * 20;

        ctx.globalAlpha = (0.22 + near * 0.3) * trace.brightness;
        ctx.drawImage(glowSprite, pulse.x - size / 2, pulse.y - size / 2, size, size);

        ctx.globalAlpha = 0.45 + near * 0.4;
        ctx.beginPath();
        ctx.arc(pulse.x, pulse.y, 0.8 + near * 0.6, 0, Math.PI * 2);
        ctx.fillStyle = "rgba(255, 238, 255, 0.92)";
        ctx.fill();
      }
      ctx.restore();
    }

    function drawPointerHalo() {
      if (!pointer.active) return;
      var size = width < 640 ? 140 : 220;
      ctx.save();
      ctx.globalCompositeOperation = "lighter";
      ctx.globalAlpha = 0.07;
      ctx.drawImage(glowSprite, pointer.x - size / 2, pointer.y - size / 2, size, size);
      ctx.restore();
    }

    function draw() {
      if (!width || !height) return;
      ctx.clearRect(0, 0, width, height);
      ctx.globalCompositeOperation = "source-over";
      ctx.globalAlpha = 1;
      drawTraces();
      drawBranches();
      if (!reduceMotion.matches) drawPulses();
      drawPointerHalo();
    }

    function frame(timestamp) {
      raf = 0;
      if (!running) return;
      if (!lastFrame) lastFrame = timestamp;
      var delta = Math.min((timestamp - lastFrame) / 1000, 0.05);
      lastFrame = timestamp;
      time += delta;
      draw();
      raf = requestAnimationFrame(frame);
    }

    function start() {
      if (running || reduceMotion.matches || document.hidden) return;
      running = true;
      lastFrame = 0;
      raf = requestAnimationFrame(frame);
    }

    function stop() {
      running = false;
      if (raf) cancelAnimationFrame(raf);
      raf = 0;
    }

    function scheduleStaticDraw() {
      if (!reduceMotion.matches || staticRaf) return;
      staticRaf = requestAnimationFrame(function () {
        staticRaf = 0;
        draw();
      });
    }

    function resizeCanvas() {
      var rect = canvas.getBoundingClientRect();
      var nextWidth = Math.max(1, Math.round(rect.width || window.innerWidth));
      var nextHeight = Math.max(1, Math.round(rect.height || window.innerHeight));
      var nextDpr = Math.min(window.devicePixelRatio || 1, MAX_DPR);

      if (nextWidth === width && nextHeight === height && nextDpr === dpr) return;

      width = nextWidth;
      height = nextHeight;
      dpr = nextDpr;
      canvas.width = Math.round(width * dpr);
      canvas.height = Math.round(height * dpr);
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      buildTraces();
      draw();
    }

    function onResize() {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(function () {
        resizeCanvas();
        start();
      }, 150);
    }

    function onPointerMove(event) {
      if (event.pointerType === "touch") return;
      pointer.x = event.clientX;
      pointer.y = event.clientY;
      pointer.active = true;
      scheduleStaticDraw();
    }

    function clearPointer() {
      pointer.active = false;
      scheduleStaticDraw();
    }

    function onVisibilityChange() {
      if (document.hidden) {
        stop();
      } else {
        start();
      }
    }

    function onMotionChange() {
      if (reduceMotion.matches) {
        stop();
        time = 0;
        draw();
      } else {
        start();
      }
    }

    resizeCanvas();
    start();

    window.addEventListener("resize", onResize, { passive: true });
    window.addEventListener("pointermove", onPointerMove, { passive: true });
    window.addEventListener("blur", clearPointer);
    document.addEventListener("mouseleave", clearPointer);
    document.addEventListener("visibilitychange", onVisibilityChange);

    if (reduceMotion.addEventListener) {
      reduceMotion.addEventListener("change", onMotionChange);
    } else if (reduceMotion.addListener) {
      reduceMotion.addListener(onMotionChange);
    }
  }

  document.addEventListener("DOMContentLoaded", initKanban);
  document.addEventListener("DOMContentLoaded", initCircuitBackground);
  document.body.addEventListener("htmx:afterSwap", initKanban);
  document.body.addEventListener("htmx:afterSettle", initKanban);
})();
