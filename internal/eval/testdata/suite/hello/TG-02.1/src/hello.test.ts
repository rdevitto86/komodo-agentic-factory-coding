import { hello } from "./hello";

test("hello greets by name", () => {
  expect(hello("Ada")).toBe("Hello, Ada!");
});
