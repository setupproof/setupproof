const assert = require("node:assert/strict");
const { greeting } = require("../src/greeting");

assert.equal(greeting("maintainer"), "hello, maintainer");
