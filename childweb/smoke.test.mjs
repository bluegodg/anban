import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const indexHTML = await readFile(new URL('./index.html', import.meta.url), 'utf8');
const appJS = await readFile(new URL('./app.js', import.meta.url), 'utf8').catch(() => '');

test('P1 loads the Stitch application through an ES module entrypoint', () => {
  assert.match(indexHTML, /<script\s+type="module"\s+src="\.\/app\.js"><\/script>/);
  assert.doesNotMatch(indexHTML, /<script>\s*\/\/ ============================\s*\/\/ SPA Router/);
});

test('P1 reuses the verified web API client without rewriting it', async () => {
  const childClient = await readFile(new URL('./api/client.js', import.meta.url), 'utf8');
  const verifiedClient = await readFile(new URL('../web/api/client.js', import.meta.url), 'utf8');

  assert.equal(childClient, verifiedClient);
});

test('P1 config trims and persists backend connection settings', async () => {
  const { DEFAULT_CONFIG, loadConfig, saveConfig } = await import('./config.js');
  const values = new Map([
    ['anban_childweb_base_url', ' http://127.0.0.1:8090/ '],
    ['anban_childweb_access_code', ' demo-code '],
  ]);
  const storage = {
    getItem(key) {
      return values.has(key) ? values.get(key) : null;
    },
    setItem(key, value) {
      values.set(key, value);
    },
  };

  assert.equal(DEFAULT_CONFIG.deviceId, '9c:13:9e:8b:af:28');
  assert.deepEqual(loadConfig(storage), {
    baseURL: 'http://127.0.0.1:8090',
    accessCode: 'demo-code',
    deviceId: '9c:13:9e:8b:af:28',
  });

  const saved = saveConfig({
    baseURL: ' https://anban.example.com/ ',
    accessCode: ' next-code ',
    deviceId: ' aa:bb:cc:dd:ee:ff ',
  }, storage);

  assert.deepEqual(saved, {
    baseURL: 'https://anban.example.com',
    accessCode: 'next-code',
    deviceId: 'aa:bb:cc:dd:ee:ff',
  });
  assert.equal(values.get('anban_childweb_device_id'), 'aa:bb:cc:dd:ee:ff');
});

test('P1 unsupported features always use the required Chinese notice', async () => {
  const { NOT_IMPLEMENTED_MESSAGE, notImplemented } = await import('./not-implemented.js');
  const notices = [];

  assert.equal(NOT_IMPLEMENTED_MESSAGE, '该功能未实现');
  assert.equal(notImplemented('附件', notices.push.bind(notices)), '该功能未实现');
  assert.deepEqual(notices, ['该功能未实现']);
});

test('P1 declares detail edit state for ES module strict mode', () => {
  assert.match(appJS, /var _detailEditTarget;/);
});
