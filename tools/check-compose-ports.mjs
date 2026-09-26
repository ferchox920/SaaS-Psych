import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";

const config = JSON.parse(
  execFileSync("docker", ["compose", "config", "--format", "json"], {
    encoding: "utf8",
  }),
);

function publishedPorts(service) {
  return (config.services[service]?.ports ?? []).map((port) =>
    Number(port.published),
  );
}

assert.deepEqual(publishedPorts("postgres"), [5433]);
assert.deepEqual(publishedPorts("redis"), [6379]);
assert.deepEqual(publishedPorts("grafana"), [3001]);
assert.ok(
  Object.keys(config.services).every(
    (service) => !publishedPorts(service).includes(3000),
  ),
  "Compose must leave port 3000 available for the demo web app",
);

console.log("Local Compose ports are compatible with the demo web app.");
