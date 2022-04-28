#!/usr/bin/env python3
"""
Test program for librclone
"""

import os
import subprocess
import unittest
from rclone import *

class TestRclone(unittest.TestCase):
    """TestSuite for rclone python module"""
    shared_object = "librclone.so"

    @classmethod
    def setUpClass(cls):
        super(TestRclone, cls).setUpClass()
        cls.shared_object = "./librclone.so"
        Rclone.build(cls.shared_object)
        cls.rclone = Rclone(shared_object=cls.shared_object)

    @classmethod
    def tearDownClass(cls):
        cls.rclone.close()
        os.remove(cls.shared_object)
        super(TestRclone, cls).tearDownClass()

    def test_rpc(self):
        o = self.rclone.rpc("rc/noop", a=42, b="string", c=[1234])
        self.assertEqual(dict(a=42, b="string", c=[1234]), o)

    def test_rpc_error(self):
        try:
            o = self.rclone.rpc("rc/error", a=42, b="string", c=[1234])
        except RcloneException as e:
            self.assertEqual(e.status, 500)
            self.assertTrue(e.output["error"].startswith("arbitrary error"))
        else:
            raise ValueError("Expecting exception")

if __name__ == '__main__':
    unittest.main()
