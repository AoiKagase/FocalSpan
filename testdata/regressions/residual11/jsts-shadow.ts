function target() {}
function caller(target: () => void) {
  target();
}
