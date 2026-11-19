#!/usr/bin/env node
// Keep the one-time token response inside this process and rundevrun's stdin.
import { spawnSync } from 'node:child_process';
import { lstatSync } from 'node:fs';
import { homedir } from 'node:os';
import path from 'node:path';
import { parseArgs } from 'node:util';

let created;
let createdId;
let stored = false;
let cf;

function run(program, args, label, input) {
  const result = spawnSync(program, args, {
    encoding: 'utf8', input, timeout: 45_000, maxBuffer: 8 * 1024 * 1024,
    env: { ...process.env, PATH: `${path.dirname(process.execPath)}${path.delimiter}${process.env.PATH || ''}` },
  });
  if (result.error || result.status !== 0) {
    throw new Error(`${label} failed (${result.error?.code || result.signal || result.status}). Output withheld; inspect authentication/permissions before retrying.`);
  }
  let data;
  try { data = JSON.parse(result.stdout); }
  catch { throw new Error(`${label} returned unexpected JSON. Output withheld.`); }
  if (data?.success === false || data?.status === 'error' || data?.status === 'partial') {
    throw new Error(`${label} reported failure. Output withheld.`);
  }
  return data?.result ?? data;
}

function statIfPresent(file) {
  try { return lstatSync(file); }
  catch (error) { if (error.code === 'ENOENT') return null; throw error; }
}

try {
  let values;
  try { ({ values } = parseArgs({ options: {
    zone: { type: 'string' },
    'credential-ref': { type: 'string', default: 'cloudflare-dns-token' },
    name: { type: 'string', default: 'rundevrun DNS' },
    cf: { type: 'string', default: 'cf' },
    rundevrun: { type: 'string', default: 'rundevrun' },
    apply: { type: 'boolean', default: false },
    help: { type: 'boolean', default: false },
  } })); } catch { throw new Error('Invalid arguments; run --help.'); }
  if (values.help) {
    console.log('Usage: node create-cloudflare-token.mjs --zone DOMAIN [--credential-ref REF] [--name NAME] [--cf PATH] [--rundevrun PATH] [--apply]\nWithout --apply, only inspect and preview. OAuth login must already be complete.');
    process.exit(0);
  }
  const zoneName = values.zone?.trim().toLowerCase().replace(/\.$/, '');
  const ref = values['credential-ref'];
  if (!zoneName || !/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(ref)) {
    throw new Error('Supply --zone DOMAIN and a valid credential reference file name.');
  }
  cf = args => run(values.cf, args, `cf ${args.slice(0, 3).join(' ')}`);
  const snapshot = run(values.rundevrun, ['network', 'inspect', '--json'], 'rundevrun network inspect');
  if (!snapshot.results?.[0]?.config) throw new Error('A compatible rundevrun network CLI is required.');

  if (values.apply) {
    const root = path.join(process.env.XDG_CONFIG_HOME || path.join(homedir(), '.config'), 'rundevrun');
    for (const directory of [root, path.join(root, 'credentials')]) {
      const info = statIfPresent(directory);
      if (info && (!info.isDirectory() || (info.mode & 0o077) !== 0)) {
        throw new Error('rundevrun configuration/credential directories must be private real directories before token creation.');
      }
    }
    if (statIfPresent(path.join(root, 'credentials', ref))) {
      throw new Error('Credential reference already exists. Reuse it or explicitly rotate it with rundevrun network credential-replace; no token was created.');
    }
  }

  const zones = cf(['zones', 'list', '--name', zoneName]);
  if (!Array.isArray(zones)) throw new Error('Unexpected zone list.');
  const matches = zones.filter(zone => zone.name?.toLowerCase() === zoneName);
  if (matches.length !== 1 || matches[0].status !== 'active' || matches[0].paused) {
    throw new Error('Exactly one active, unpaused Cloudflare zone is required. Complete onboarding first.');
  }
  const zone = matches[0];
  const groups = cf(['user', 'tokens', 'permission-groups', 'list']);
  if (!Array.isArray(groups)) throw new Error('Unexpected permission group list.');
  const permissions = [['Zone Read'], ['DNS Write', 'DNS Edit']].map(names => {
    for (const name of names) {
      const found = groups.filter(group => group.name === name && group.scopes?.includes('com.cloudflare.api.account.zone'));
      if (found.length === 1) return { id: found[0].id, name };
      if (found.length > 1) break;
    }
    throw new Error('Cannot identify the exact zone-scoped DNS/Zone permission groups.');
  });
  const policy = { name: values.name, policies: [{
    effect: 'allow', permission_groups: permissions.map(({ id }) => ({ id })),
    resources: { [`com.cloudflare.api.account.zone.${zone.id}`]: '*' },
  }] };
  const summary = { zone: zone.name, zone_id: zone.id, credential_ref: ref, permissions };
  if (!values.apply) {
    console.log(JSON.stringify({ status: 'planned', ...summary, policy, expires_on: null }, null, 2));
  } else {
    try { created = cf(['user', 'tokens', 'create', '--body', JSON.stringify(policy)]); }
    catch { throw new Error('Token creation has no confirmed result. Inspect the requested token name in Cloudflare before retrying; output withheld.'); }
    createdId = typeof created?.id === 'string' && /^[a-f0-9]{32}$/i.test(created.id) ? created.id : undefined;
    if (!createdId || typeof created?.value !== 'string' || !created.value) {
      throw new Error('Token creation response is incomplete. Inspect tokens with the requested name before retrying; response withheld.');
    }
    const result = run(values.rundevrun, ['network', 'credential-store', ref, '--json'], 'rundevrun credential-store', created.value);
    if (result.results?.[0]?.credential_ref !== ref) throw new Error('rundevrun did not confirm the expected credential reference.');
    stored = true;
    delete created.value;
    console.log(JSON.stringify({ status: 'stored', ...summary, token_id: createdId, expires_on: created.expires_on ?? null }, null, 2));
  }
} catch (error) {
  let recovery = '';
  if (createdId && !stored) {
    try {
      cf(['user', 'tokens', 'delete', createdId, '--force']);
      recovery = ' Newly created token revoked. Inspect the credential reference before retrying.';
    } catch {
      recovery = ` Newly created token ${createdId} could not be revoked; revoke that token in Cloudflare before retrying.`;
    }
  }
  console.error(`${error.message}${recovery}`);
  process.exitCode = 1;
}
