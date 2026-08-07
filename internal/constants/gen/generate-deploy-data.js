const fs = require('fs');
const path = require('path');
const { ethers } = require('ethers');

const TS_DEPLOY_DATA = path.resolve(__dirname, '../../../shared/common/src/constants/deploy-data');
const GO_DEPLOY_DATA = path.resolve(__dirname, '../deploy-data');

const toJsonAbi = (humanReadableAbi) => JSON.parse(new ethers.Interface(humanReadableAbi).formatJson());

const readJson = (file) => JSON.parse(fs.readFileSync(file, 'utf8'));

const writeJson = (file, value) => fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);

for (const file of fs.readdirSync(TS_DEPLOY_DATA)) {
  if (!file.startsWith('deploy-data-') || !file.endsWith('.json')) continue;
  fs.copyFileSync(path.join(TS_DEPLOY_DATA, file), path.join(GO_DEPLOY_DATA, file));
}

// Convert the shared ABIs (human-readable -> JSON) that go-ethereum needs to pack calls.
// hinkalHelperABI is intentionally omitted; the Go SDK does not use it.
for (const family of ['evm', 'tron']) {
  const shared = readJson(path.join(TS_DEPLOY_DATA, `shared-deploy-data-${family}.json`));
  writeJson(path.join(GO_DEPLOY_DATA, `shared-deploy-data-${family}.json`), {
    hinkalABI: toJsonAbi(shared.hinkalABI),
    hinkalWrapperABI: toJsonAbi(shared.hinkalWrapperABI),
  });
}

console.log('Regenerated Go deploy data from', path.relative(process.cwd(), TS_DEPLOY_DATA));
