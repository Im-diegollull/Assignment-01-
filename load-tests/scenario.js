import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';
import execution from 'k6/execution';

const count = Number(__ENV.REQUEST_COUNT || 10);
const seconds = Number(__ENV.DURATION_SECONDS || 300);
const statuses = new Counter('response_status');

export const options = {
  insecureSkipTLSVerify: true,
  discardResponseBodies: true,
  maxRedirects: 0,
  scenarios: {
    endpoint: {
      executor: 'constant-arrival-rate',
      rate: count,
      timeUnit: `${seconds}s`,
      duration: `${seconds}s`,
      preAllocatedVUs: 20,
      maxVUs: 100,
      gracefulStop: '30s',
    },
  },
  tags: {
    deployment: __ENV.DEPLOYMENT,
    endpoint: __ENV.ENDPOINT,
    request_count: String(count),
  },
  thresholds: {
    http_req_failed: ['rate==0'],
    checks: ['rate==1'],
    dropped_iterations: ['count==0'],
    http_reqs: [`count==${count}`],
  },
};

export default function () {
  if (execution.scenario.iterationInTest >= count) return;
  const response = http.get(`${__ENV.BASE_URL || 'https://app.localhost'}${__ENV.ENDPOINT_PATH}`, { timeout: '15s' });
  statuses.add(1, { status: String(response.status) });
  check(response, { 'HTTP 200': (r) => r.status === 200 });
}
