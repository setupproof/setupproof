const assert = require("node:assert/strict");
const { add } = require("../src/add");

assert.equal(add(2, 3), 5);
assert.equal(add(-1, 1), 0);
