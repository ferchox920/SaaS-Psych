import socket
import unittest

from benchmark_local_asr import no_network


class LocalASRNetworkTest(unittest.TestCase):
    def test_offline_guard_blocks_tcp_and_udp_and_restores_socket_methods(self):
        original = socket.socket.connect
        with no_network() as attempts:
            with socket.socket() as tcp:
                with self.assertRaisesRegex(RuntimeError, "Network access is disabled"):
                    tcp.connect(("127.0.0.1", 9))
                with self.assertRaises(RuntimeError):
                    tcp.connect_ex(("127.0.0.1", 9))
            with socket.socket(type=socket.SOCK_DGRAM) as udp:
                with self.assertRaises(RuntimeError):
                    udp.sendto(b"synthetic", ("127.0.0.1", 9))
            self.assertEqual(attempts, ["connect", "connect_ex", "sendto"])
        self.assertIs(socket.socket.connect, original)

    def test_socket_guard_is_restored_when_inference_raises(self):
        original = socket.socket.connect
        with self.assertRaises(ValueError):
            with no_network():
                raise ValueError("synthetic model failure")
        self.assertIs(socket.socket.connect, original)


if __name__ == "__main__":
    unittest.main()
