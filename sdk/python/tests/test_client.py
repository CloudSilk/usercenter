import json
import unittest
from unittest.mock import patch, MagicMock


class TestUserCenterClient(unittest.TestCase):

    def setUp(self):
        from usercenter_client.client import UserCenterClient
        self.client = UserCenterClient("http://localhost:48080")

    @patch("usercenter_client.client.requests.Session.request")
    def test_login(self, mock_request):
        mock_resp = MagicMock()
        mock_resp.json.return_value = {"code": 0, "data": "jwt_token_abc"}
        mock_request.return_value = mock_resp

        token = self.client.login("admin", "password123")

        mock_request.assert_called_once()
        args, kwargs = mock_request.call_args
        self.assertEqual(kwargs["method"], "POST")
        self.assertIn("/api/core/auth/user/login", kwargs["url"])
        self.assertEqual(kwargs["json"], {"userName": "admin", "password": "password123"})
        self.assertEqual(token, "jwt_token_abc")
        self.assertEqual(self.client.session.headers["Authorization"], "Bearer jwt_token_abc")

    @patch("usercenter_client.client.requests.Session.request")
    def test_chat_completion(self, mock_request):
        mock_resp = MagicMock()
        mock_resp.text = json.dumps({
            "id": "cmpl-xxx",
            "choices": [{"message": {"content": "Hello!"}}],
        })
        mock_request.return_value = mock_resp

        result = self.client.chat_completion(
            "gpt-3.5-turbo",
            [{"role": "user", "content": "hi"}],
            stream=False,
        )

        args, kwargs = mock_request.call_args
        self.assertEqual(kwargs["method"], "POST")
        self.assertIn("/v1/chat/completions", kwargs["url"])
        payload = kwargs["json"]
        self.assertEqual(payload["model"], "gpt-3.5-turbo")
        self.assertEqual(payload["messages"], [{"role": "user", "content": "hi"}])
        self.assertIs(payload["stream"], False)
        self.assertIsInstance(result, str)

    @patch("usercenter_client.client.requests.Session.request")
    def test_add_menu(self, mock_request):
        mock_resp = MagicMock()
        mock_resp.json.return_value = {"code": 0, "message": "success"}
        mock_request.return_value = mock_resp

        result = self.client.add_menu({"name": "system", "path": "/system"})

        args, kwargs = mock_request.call_args
        self.assertEqual(kwargs["method"], "POST")
        self.assertIn("/api/core/auth/menu/add", kwargs["url"])
        self.assertEqual(kwargs["json"]["name"], "system")

    @patch("usercenter_client.client.requests.Session.request")
    def test_oauth_token(self, mock_request):
        mock_resp = MagicMock()
        mock_resp.text = json.dumps({"access_token": "abc", "token_type": "Bearer"})
        mock_request.return_value = mock_resp

        result = self.client.oauth_token("code123", "http://localhost/callback", "client1", "secret1")

        args, kwargs = mock_request.call_args
        self.assertEqual(kwargs["method"], "POST")
        self.assertIn("/oauth/token", kwargs["url"])

    @patch("usercenter_client.client.requests.Session.request")
    def test_health(self, mock_request):
        mock_resp = MagicMock()
        mock_resp.status_code = 200
        mock_request.return_value = mock_resp

        self.assertTrue(self.client.health())


if __name__ == "__main__":
    unittest.main()
