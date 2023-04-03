import http from 'k6/http';

export const options = {
  vus: 10,
  duration: '10s',
  thresholds: {
    'http_reqs{expected_response:true}': ['rate>5'],
  },
};

export default function () {
  http.get('https://test.k6.io/');
}
