// @ts-expect-error

export function foo() {
  const x = bar(); // eslint-disable-line no-unused-vars
  return x;
}
