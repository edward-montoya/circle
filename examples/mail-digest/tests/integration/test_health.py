import threading
import unittest
import urllib.request

from app.server import make_server


class HealthTest(unittest.TestCase):
    def setUp(self):
        self.server = make_server("127.0.0.1", 0)
        self.port = self.server.server_address[1]
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()

    def test_health_answers_200(self):
        with urllib.request.urlopen(f"http://127.0.0.1:{self.port}/health") as response:
            self.assertEqual(response.status, 200)

    def test_unknown_path_answers_404(self):
        with self.assertRaises(urllib.error.HTTPError) as caught:
            urllib.request.urlopen(f"http://127.0.0.1:{self.port}/nope")
        self.assertEqual(caught.exception.code, 404)
