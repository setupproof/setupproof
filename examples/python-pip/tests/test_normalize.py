import unittest

from setupproof_pip_example import normalize


class NormalizeTests(unittest.TestCase):
    def test_normalize_spaces_and_case(self):
        self.assertEqual(normalize(" Setup Proof "), "setup-proof")


if __name__ == "__main__":
    unittest.main()
