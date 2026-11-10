// A small CSS 3D scene. Pointer movement is optional; no renderer or animation library.
export function initializeLoginScene() {
  const scene = document.querySelector(".login-scene");
  const stage = scene.querySelector(".scene-stage");
  const motion = matchMedia("(prefers-reduced-motion: reduce)");
  const pointer = matchMedia("(hover: hover) and (pointer: fine)");
  let frame = null;

  function reset() {
    cancelAnimationFrame(frame);
    stage.style.setProperty("--tilt-x", "0deg");
    stage.style.setProperty("--tilt-y", "0deg");
  }

  scene.addEventListener("pointermove", (event) => {
    if (motion.matches || !pointer.matches) return;
    const bounds = scene.getBoundingClientRect();
    const horizontal = (event.clientX - bounds.left) / bounds.width - 0.5;
    const vertical = (event.clientY - bounds.top) / bounds.height - 0.5;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(() => {
      stage.style.setProperty("--tilt-x", (-vertical * 9).toFixed(2) + "deg");
      stage.style.setProperty("--tilt-y", (horizontal * 12).toFixed(2) + "deg");
    });
  });
  scene.addEventListener("pointerleave", reset);
  motion.addEventListener("change", reset);
}
