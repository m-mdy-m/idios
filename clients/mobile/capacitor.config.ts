import type { CapacitorConfig } from '@capacitor/cli';

const rawServerURL = process.env.IDIOS_MOBILE_SERVER_URL?.trim();
if (!rawServerURL) {
  throw new Error(
    'IDIOS_MOBILE_SERVER_URL is required. Use an HTTPS IDIOS endpoint protected by your deployment authentication.',
  );
}

let serverURL: URL;
try {
  serverURL = new URL(rawServerURL);
} catch {
  throw new Error('IDIOS_MOBILE_SERVER_URL must be a valid absolute URL.');
}

if (
  serverURL.protocol !== 'https:' ||
  !serverURL.hostname ||
  serverURL.username ||
  serverURL.password ||
  serverURL.search ||
  serverURL.hash
) {
  throw new Error(
    'IDIOS_MOBILE_SERVER_URL must be an HTTPS URL without credentials, query parameters, or a fragment.',
  );
}

const config: CapacitorConfig = {
  appId: 'com.idios.app',
  appName: 'IDIOS',
  webDir: '../web/dist',
  server: {
    url: serverURL.toString().replace(/\/$/, ''),
    cleartext: false,
    allowNavigation: [serverURL.hostname],
  },
};

export default config;
