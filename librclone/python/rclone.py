"""
Python interface to librclone.so using ctypes

Create an rclone object

    rclone = Rclone(shared_object="/path/to/librclone.so")

Then call rpc calls on it

    rclone.rpc("rc/noop", a=42, b="string", c=[1234])

When finished, close it

    rclone.close()
"""

__all__ = ('Rclone', 'RcloneException')

import os
import json
import subprocess
from ctypes import *

class RcloneRPCResult(Structure):
    """
    This is returned from the C API when calling RcloneRPC
    """
    _fields_ = [("Output", c_char_p),
                ("Status", c_int)]

class RcloneException(Exception):
    """
    Exception raised from rclone

    This will have the attributes:

    output - a dictionary from the call
    status - a status number
    """
    def __init__(self, output, status):
        self.output = output
        self.status = status
        message = self.output.get('error', 'Unknown rclone error')
        super().__init__(message)

class Rclone():
    """
    Interface to Rclone via librclone.so

    Initialise with shared_object as the file path of librclone.so
    """
    def __init__(self, shared_object="./librclone.so"):
        self.rclone = CDLL(shared_object)
        self.rclone.RcloneRPC.restype = RcloneRPCResult
        self.rclone.RcloneRPC.argtypes = (c_char_p, c_char_p)
        self.rclone.RcloneInitialize.restype = None
        self.rclone.RcloneInitialize.argtypes = ()
        self.rclone.RcloneFinalize.restype = None
        self.rclone.RcloneFinalize.argtypes = ()
        self.rclone.RcloneInitialize()
    def rpc(self, method, **kwargs):
        """
        Call an rclone RC API call with the kwargs given.

        The result will be a dictionary.

        If an exception is raised from rclone it will of type
        RcloneException.
        """
        method = method.encode("utf-8")
        parameters = json.dumps(kwargs).encode("utf-8")
        resp = self.rclone.RcloneRPC(method, parameters)
        output = json.loads(resp.Output.decode("utf-8"))
        status = resp.Status
        if status != 200:
            raise RcloneException(output, status)
        return output
    def close(self):
        """
        Call to finish with the rclone connection
        """
        self.rclone.RcloneFinalize()
        self.rclone = None
    @classmethod
    def build(cls, shared_object):
        """
        Builds rclone to shared_object if it doesn't already exist

        Requires go to be installed
        """
        if os.path.exists(shared_object):
            return
        print("Building "+shared_object)
        subprocess.check_call(["go", "build", "--buildmode=c-shared", "-o", shared_object, "github.com/rclone/rclone/librclone"])
