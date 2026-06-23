const { spawnSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const rootDir = path.resolve(__dirname, '..');
const configPath = path.join(rootDir, 'next.config.js');
const prodConfigPath = path.join(rootDir, 'next.config.prod.js');
const nextCli = path.join(rootDir, 'node_modules', 'next', 'dist', 'bin', 'next');

const originalConfig = fs.existsSync(configPath)
  ? fs.readFileSync(configPath)
  : undefined;

try {
  fs.copyFileSync(prodConfigPath, configPath);

  const result = spawnSync(process.execPath, [nextCli, 'build'], {
    cwd: rootDir,
    env: process.env,
    stdio: 'inherit',
  });

  if (result.error) {
    throw result.error;
  }
  process.exitCode = result.status || 0;
} finally {
  if (originalConfig === undefined) {
    fs.rmSync(configPath, { force: true });
  } else {
    fs.writeFileSync(configPath, originalConfig);
  }
}
