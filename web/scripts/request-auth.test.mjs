import test from 'node:test';
import assert from 'node:assert/strict';

import { shouldAttachAuthHeader } from '../src/lib/requestAuth.mjs';

test('does not attach admin token to third-party absolute URLs', () => {
  assert.equal(
    shouldAttachAuthHeader('https://example.test/api', '/api/v1', 'http://localhost:3000'),
    false,
  );
});

test('attaches admin token to relative API URLs', () => {
  assert.equal(
    shouldAttachAuthHeader('/api/v1/videos', '/api/v1', 'http://localhost:3000'),
    true,
  );
});

test('attaches admin token to same-origin absolute URLs', () => {
  assert.equal(
    shouldAttachAuthHeader('http://localhost:3000/api/v1/videos', '/api/v1', 'http://localhost:3000'),
    true,
  );
});

test('attaches admin token to configured absolute API base URL', () => {
  assert.equal(
    shouldAttachAuthHeader(
      'http://localhost:8096/api/v1/videos',
      'http://localhost:8096/api/v1',
      'http://localhost:3000',
    ),
    true,
  );
});
