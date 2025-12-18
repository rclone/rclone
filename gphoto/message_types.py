"""Protobuf message types"""

COMMIT_UPLOAD = {
    "1": {
        "type": "message",
        "message_typedef": {
            "1": {"type": "message", "message_typedef": {"1": {"type": "int"}, "2": {"type": "bytes"}}},
            "2": {"type": "string"},
            "3": {"type": "bytes"},
            "4": {"type": "message", "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}},
            "7": {"type": "int"},
            "8": {
                "type": "message",
                "message_typedef": {
                    "1": {
                        "type": "message",
                        "message_typedef": {
                            "1": {"type": "string"},
                            "3": {"type": "string"},
                            "4": {"type": "string"},
                            "5": {"type": "message", "message_typedef": {"1": {"type": "string"}, "2": {"type": "string"}, "3": {"type": "string"}, "4": {"type": "string"}, "5": {"type": "string"}, "7": {"type": "string"}}},
                            "6": {"type": "string"},
                            "7": {"type": "message", "message_typedef": {"2": {"type": "string"}}},
                            "15": {"type": "string"},
                            "16": {"type": "string"},
                            "17": {"type": "string"},
                            "19": {"type": "string"},
                            "20": {"type": "string"},
                            "21": {"type": "message", "message_typedef": {"5": {"type": "message", "message_typedef": {"3": {"type": "string"}}}, "6": {"type": "string"}}},
                            "25": {"type": "string"},
                            "30": {"type": "message", "message_typedef": {"2": {"type": "string"}}},
                            "31": {"type": "string"},
                            "32": {"type": "string"},
                            "33": {"type": "message", "message_typedef": {"1": {"type": "string"}}},
                            "34": {"type": "string"},
                            "36": {"type": "string"},
                            "37": {"type": "string"},
                            "38": {"type": "string"},
                            "39": {"type": "string"},
                            "40": {"type": "string"},
                            "41": {"type": "string"},
                        },
                    },
                    "5": {
                        "type": "message",
                        "message_typedef": {
                            "2": {
                                "type": "message",
                                "message_typedef": {
                                    "2": {"type": "message", "message_typedef": {"3": {"type": "message", "message_typedef": {"2": {"type": "string"}}}, "4": {"type": "message", "message_typedef": {"2": {"type": "string"}}}}},
                                    "4": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"2": {"type": "int"}}}}},
                                    "5": {"type": "message", "message_typedef": {"2": {"type": "string"}}},
                                    "6": {"type": "int"},
                                },
                            },
                            "3": {
                                "type": "message",
                                "message_typedef": {
                                    "2": {"type": "message", "message_typedef": {"3": {"type": "string"}, "4": {"type": "string"}}},
                                    "3": {"type": "message", "message_typedef": {"2": {"type": "string"}, "3": {"type": "message", "message_typedef": {"2": {"type": "int"}}}}},
                                    "4": {"type": "string"},
                                    "5": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"2": {"type": "int"}}}}},
                                    "7": {"type": "string"},
                                },
                            },
                            "4": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"2": {"type": "string"}}}}},
                            "5": {
                                "type": "message",
                                "message_typedef": {
                                    "1": {
                                        "type": "message",
                                        "message_typedef": {
                                            "2": {"type": "message", "message_typedef": {"3": {"type": "string"}, "4": {"type": "string"}}},
                                            "3": {"type": "message", "message_typedef": {"2": {"type": "string"}, "3": {"type": "message", "message_typedef": {"2": {"type": "int"}}}}},
                                        },
                                    },
                                    "3": {"type": "int"},
                                },
                            },
                        },
                    },
                    "8": {"type": "string"},
                    "9": {
                        "type": "message",
                        "message_typedef": {
                            "2": {"type": "string"},
                            "3": {"type": "message", "message_typedef": {"1": {"type": "string"}, "2": {"type": "string"}}},
                            "4": {
                                "type": "message",
                                "message_typedef": {
                                    "1": {
                                        "type": "message",
                                        "message_typedef": {
                                            "3": {
                                                "type": "message",
                                                "message_typedef": {
                                                    "1": {
                                                        "type": "message",
                                                        "message_typedef": {
                                                            "1": {"type": "message", "message_typedef": {"5": {"type": "message", "message_typedef": {"1": {"type": "string"}}}, "6": {"type": "string"}}},
                                                            "2": {"type": "string"},
                                                            "3": {
                                                                "type": "message",
                                                                "message_typedef": {
                                                                    "1": {"type": "message", "message_typedef": {"5": {"type": "message", "message_typedef": {"1": {"type": "string"}}}, "6": {"type": "string"}}},
                                                                    "2": {"type": "string"},
                                                                },
                                                            },
                                                        },
                                                    }
                                                },
                                            },
                                            "4": {"type": "message", "message_typedef": {"1": {"type": "message", "message_typedef": {"2": {"type": "string"}}}}},
                                        },
                                    }
                                },
                            },
                        },
                    },
                    "11": {
                        "type": "message",
                        "message_typedef": {"2": {"type": "string"}, "3": {"type": "string"}, "4": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}}}}},
                    },
                    "12": {"type": "string"},
                    "14": {
                        "type": "message",
                        "message_typedef": {"2": {"type": "string"}, "3": {"type": "string"}, "4": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}}}}},
                    },
                    "15": {"type": "message", "message_typedef": {"1": {"type": "string"}, "4": {"type": "string"}}},
                    "17": {"type": "message", "message_typedef": {"1": {"type": "string"}, "4": {"type": "string"}}},
                    "19": {
                        "type": "message",
                        "message_typedef": {"2": {"type": "string"}, "3": {"type": "string"}, "4": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}}}}},
                    },
                    "22": {"type": "string"},
                    "23": {"type": "string"},
                },
            },
            "10": {"type": "int"},
            "17": {"type": "int"},
        },
    },
    "2": {"type": "message", "message_typedef": {"3": {"type": "string"}, "4": {"type": "string"}, "5": {"type": "int"}}},
    "3": {"type": "bytes"},
}

CREATE_ALBUM = {
    "1": {"type": "string"},
    "2": {"type": "int"},
    "3": {"type": "int"},
    "4": {"seen_repeated": True, "field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"type": "string"}}, "type": "message"}}, "type": "message"},
    "6": {"message_typedef": {}, "type": "message"},
    "7": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
    "8": {"field_order": ["3", "4", "5"], "message_typedef": {"3": {"type": "string"}, "4": {"type": "string"}, "5": {"type": "int"}}, "type": "message"},
}

MOVE_TO_TRASH = {
    "2": {"type": "int"},
    "3": {"type": "string"},
    "4": {"type": "int"},
    "8": {
        "field_order": ["4"],
        "message_typedef": {
            "4": {
                "field_order": ["2", "3", "4", "5"],
                "message_typedef": {
                    "2": {"message_typedef": {}, "type": "message"},
                    "3": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"message_typedef": {}, "type": "message"},
                    "5": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                },
                "type": "message",
            }
        },
        "type": "message",
    },
    "9": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"}}, "type": "message"},
}

FIND_REMOTE_MEDIA_BY_HASH = {
    "1": {
        "field_order": ["1", "2"],
        "message_typedef": {
            "1": {
                "field_order": ["1"],
                "message_typedef": {
                    "1": {"type": "bytes"},
                },
                "type": "message",
            },
            "2": {"message_typedef": {}, "type": "message"},
        },
        "type": "message",
    },
}

GET_UPLOAD_TOKEN = {
    "1": {"type": "int"},
    "2": {"type": "int"},
    "3": {"type": "int"},
    "4": {"type": "int"},
    "7": {"type": "int"},
}

ADD_MEDIA_TO_ALBUM = {
    "1": {"type": "string"},
    "2": {"type": "string"},
    "5": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
    "6": {
        "field_order": ["3", "4", "5"],
        "message_typedef": {
            "3": {"type": "string"},
            "4": {"type": "string"},
            "5": {"type": "int"},
        },
        "type": "message",
    },
    "7": {"type": "int"},
}

GET_LIB_PAGE_INIT = {
    "1": {
        "field_order": ["1", "2", "3", "4", "7", "9", "11", "11", "12", "13", "15", "18", "19", "20", "21", "22", "25"],
        "message_typedef": {
            "1": {
                "field_order": ["1", "5", "8", "9", "11", "12", "14", "15", "17", "19", "22", "23"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "3", "4", "5", "6", "7", "15", "16", "17", "19", "20", "21", "25", "30", "31", "32", "33", "34", "36", "37", "38", "39", "40", "41"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "6": {"message_typedef": {}, "type": "message"},
                            "7": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"message_typedef": {}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {
                                "field_order": ["5", "6"],
                                "message_typedef": {"5": {"field_order": ["3"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "6": {"message_typedef": {}, "type": "message"}},
                                "type": "message",
                            },
                            "25": {"message_typedef": {}, "type": "message"},
                            "30": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "33": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "36": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "40": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "5": {
                        "field_order": ["2", "3", "4", "5"],
                        "message_typedef": {
                            "2": {
                                "field_order": ["2", "4", "5", "6"],
                                "message_typedef": {
                                    "2": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    },
                                    "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "6": {"type": "int"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                            "5": {
                                "field_order": ["1", "3"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["2", "3"],
                                        "message_typedef": {
                                            "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "3": {
                                                "field_order": ["2", "3"],
                                                "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}},
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "3": {"type": "int"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "8": {"message_typedef": {}, "type": "message"},
                    "9": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "4": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {
                                                "field_order": ["1"],
                                                "message_typedef": {
                                                    "1": {
                                                        "field_order": ["1", "2", "3"],
                                                        "message_typedef": {
                                                            "1": {
                                                                "field_order": ["5", "6"],
                                                                "message_typedef": {
                                                                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                    "6": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                            "2": {"message_typedef": {}, "type": "message"},
                                                            "3": {
                                                                "field_order": ["1", "2"],
                                                                "message_typedef": {
                                                                    "1": {
                                                                        "field_order": ["5", "6"],
                                                                        "message_typedef": {
                                                                            "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                            "6": {"message_typedef": {}, "type": "message"},
                                                                        },
                                                                        "type": "message",
                                                                    },
                                                                    "2": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                        },
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "4": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "11": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "12": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "15": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "17": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "19": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "22": {"message_typedef": {}, "type": "message"},
                    "23": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "2": {
                "field_order": ["1", "4", "9", "11", "14", "17", "18", "20", "23"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2", "3", "4", "5", "6", "7", "8", "10", "12", "13", "15", "18"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                            "12": {"message_typedef": {}, "type": "message"},
                            "13": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                    "11": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "4", "5", "6", "9"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "6": {"message_typedef": {}, "type": "message"},
                                    "9": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "14": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "2"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1", "2", "3"],
                                        "message_typedef": {
                                            "1": {"message_typedef": {}, "type": "message"},
                                            "2": {
                                                "field_order": ["2"],
                                                "message_typedef": {
                                                    "2": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "3": {
                                                "field_order": ["4", "5"],
                                                "message_typedef": {
                                                    "4": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                    "5": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                },
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "2": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "17": {"message_typedef": {}, "type": "message"},
                    "18": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "20": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "23": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "3": {
                "field_order": ["2", "3", "4", "7", "12", "13", "14", "15", "16", "18", "19", "20", "24", "25"],
                "message_typedef": {
                    "2": {"message_typedef": {}, "type": "message"},
                    "3": {
                        "field_order": ["2", "3", "7", "8", "14", "16", "17", "18", "19", "20", "21", "22", "23", "27", "29", "30", "31", "32", "34", "37", "38", "39", "41"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "14": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {"message_typedef": {}, "type": "message"},
                            "22": {"message_typedef": {}, "type": "message"},
                            "23": {"message_typedef": {}, "type": "message"},
                            "27": {
                                "field_order": ["1", "2"],
                                "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                                "type": "message",
                            },
                            "29": {"message_typedef": {}, "type": "message"},
                            "30": {"message_typedef": {}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"field_order": ["2", "3", "4"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "7": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["1", "2", "3"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "2": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "15": {"message_typedef": {}, "type": "message"},
                    "16": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "18": {"message_typedef": {}, "type": "message"},
                    "19": {
                        "field_order": ["4", "6", "7", "8"],
                        "message_typedef": {
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "6": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "20": {"message_typedef": {}, "type": "message"},
                    "24": {"message_typedef": {}, "type": "message"},
                    "25": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "4": {"type": "string"},
            "7": {"type": "int"},
            "9": {
                "field_order": ["1", "2", "3", "4", "7", "8", "9"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                    "7": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "11": {"seen_repeated": True, "type": "int"},
            "12": {
                "field_order": ["2", "3", "4"],
                "message_typedef": {
                    "2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "13": {"message_typedef": {}, "type": "message"},
            "15": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"},
            "18": {
                "field_order": ["169945741"],
                "message_typedef": {
                    "169945741": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "19": {
                "field_order": ["1", "2", "3", "5", "6", "7", "8"],
                "message_typedef": {
                    "1": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "2": {"field_order": ["1", "1", "1", "1", "1", "1"], "message_typedef": {"1": {"seen_repeated": True, "type": "int"}}, "type": "message"},
                    "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "5": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "7": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                },
                "type": "message",
            },
            "20": {
                "field_order": ["1", "3"],
                "message_typedef": {
                    "1": {"type": "int"},
                    "3": {
                        "field_order": ["1", "2"],
                        "message_typedef": {
                            "1": {"type": "string"},
                            "2": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                },
                "type": "message",
            },
            "21": {
                "field_order": ["2", "3", "5", "6", "7", "8", "9", "10", "11", "12", "13"],
                "message_typedef": {
                    "2": {"field_order": ["2", "4", "5"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}, "5": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"},
                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "7": {"field_order": ["1", "2", "3"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}, "3": {"type": "string"}}, "type": "message"},
                    "8": {
                        "field_order": ["3", "4"],
                        "message_typedef": {
                            "3": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1"],
                                        "message_typedef": {"1": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"}},
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "9": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "10": {
                        "field_order": ["1", "3", "5", "6", "7", "9", "10"],
                        "message_typedef": {
                            "1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "9": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "11": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "22": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
            "25": {
                "field_order": ["1", "2"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
        },
        "type": "message",
    },
    "2": {
        "field_order": ["1", "2"],
        "message_typedef": {
            "1": {
                "field_order": ["1"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}},
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "2": {"message_typedef": {}, "type": "message"},
        },
        "type": "message",
    },
}

GET_LIB_STATE = {
    "1": {
        "field_order": ["1", "2", "3", "6", "7", "9", "11", "11", "11", "12", "13", "15", "18", "19", "20", "21", "22", "25", "26"],
        "message_typedef": {
            "1": {
                "field_order": ["1", "5", "8", "9", "11", "12", "14", "15", "17", "19", "21", "22", "23", "24"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "3", "4", "5", "6", "7", "15", "16", "17", "19", "20", "21", "25", "30", "31", "32", "33", "34", "36", "37", "38", "39", "40", "41"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "6": {"message_typedef": {}, "type": "message"},
                            "7": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"message_typedef": {}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {
                                "field_order": ["5", "6"],
                                "message_typedef": {"5": {"field_order": ["3"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "6": {"message_typedef": {}, "type": "message"}},
                                "type": "message",
                            },
                            "25": {"message_typedef": {}, "type": "message"},
                            "30": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "33": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "36": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "40": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "5": {
                        "field_order": ["2", "3", "4", "5"],
                        "message_typedef": {
                            "2": {
                                "field_order": ["2", "4", "5", "6"],
                                "message_typedef": {
                                    "2": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "4": {"field_order": ["2", "4"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    },
                                    "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "6": {"type": "int"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {
                                        "field_order": ["2", "3"],
                                        "message_typedef": {
                                            "2": {"message_typedef": {}, "type": "message"},
                                            "3": {"field_order": ["2", "3"], "message_typedef": {"2": {"type": "int"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    },
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                            "5": {
                                "field_order": ["1", "3"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["2", "3"],
                                        "message_typedef": {
                                            "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "3": {
                                                "field_order": ["2", "3"],
                                                "message_typedef": {
                                                    "2": {"message_typedef": {}, "type": "message"},
                                                    "3": {"field_order": ["2", "3"], "message_typedef": {"2": {"type": "int"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                },
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "3": {"type": "int"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "8": {"message_typedef": {}, "type": "message"},
                    "9": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "4": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {
                                                "field_order": ["1"],
                                                "message_typedef": {
                                                    "1": {
                                                        "field_order": ["1", "2", "3"],
                                                        "message_typedef": {
                                                            "1": {
                                                                "field_order": ["5", "6", "7"],
                                                                "message_typedef": {
                                                                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                    "6": {"message_typedef": {}, "type": "message"},
                                                                    "7": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                            "2": {"message_typedef": {}, "type": "message"},
                                                            "3": {
                                                                "field_order": ["1", "2"],
                                                                "message_typedef": {
                                                                    "1": {
                                                                        "field_order": ["5", "6", "7"],
                                                                        "message_typedef": {
                                                                            "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                            "6": {"message_typedef": {}, "type": "message"},
                                                                            "7": {"message_typedef": {}, "type": "message"},
                                                                        },
                                                                        "type": "message",
                                                                    },
                                                                    "2": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                        },
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "4": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "11": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "12": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "15": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "17": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "19": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "21": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "22": {"message_typedef": {}, "type": "message"},
                    "23": {"message_typedef": {}, "type": "message"},
                    "24": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "2": {
                "field_order": ["1", "4", "9", "11", "14", "17", "18", "20", "22", "23", "24"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2", "3", "4", "5", "6", "7", "8", "10", "12", "13", "15", "18"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                            "12": {"message_typedef": {}, "type": "message"},
                            "13": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                    "11": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "4", "5", "6", "9"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "6": {"message_typedef": {}, "type": "message"},
                                    "9": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "14": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "2"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1", "2", "3"],
                                        "message_typedef": {
                                            "1": {"message_typedef": {}, "type": "message"},
                                            "2": {
                                                "field_order": ["2"],
                                                "message_typedef": {
                                                    "2": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "3": {
                                                "field_order": ["4", "5"],
                                                "message_typedef": {
                                                    "4": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                    "5": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                },
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "2": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "17": {"message_typedef": {}, "type": "message"},
                    "18": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "20": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "22": {"message_typedef": {}, "type": "message"},
                    "23": {"message_typedef": {}, "type": "message"},
                    "24": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "3": {
                "field_order": ["2", "3", "4", "7", "12", "13", "14", "15", "16", "18", "19", "20", "22", "24", "25", "26"],
                "message_typedef": {
                    "2": {"message_typedef": {}, "type": "message"},
                    "3": {
                        "field_order": ["2", "3", "7", "8", "14", "16", "17", "18", "19", "20", "21", "22", "23", "27", "29", "30", "31", "32", "34", "37", "38", "39", "41", "43", "45", "46", "47"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "14": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {"message_typedef": {}, "type": "message"},
                            "22": {"message_typedef": {}, "type": "message"},
                            "23": {"message_typedef": {}, "type": "message"},
                            "27": {
                                "field_order": ["1", "2"],
                                "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                                "type": "message",
                            },
                            "29": {"message_typedef": {}, "type": "message"},
                            "30": {"message_typedef": {}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                            "43": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "45": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                            "46": {
                                "field_order": ["1", "2", "3"],
                                "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                "type": "message",
                            },
                            "47": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {
                        "field_order": ["2", "3", "4", "5"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "7": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["1", "2", "3"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "2": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "15": {"message_typedef": {}, "type": "message"},
                    "16": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "18": {"message_typedef": {}, "type": "message"},
                    "19": {
                        "field_order": ["4", "6", "7", "8", "9"],
                        "message_typedef": {
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "6": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "9": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "20": {"message_typedef": {}, "type": "message"},
                    "22": {"message_typedef": {}, "type": "message"},
                    "24": {"message_typedef": {}, "type": "message"},
                    "25": {"message_typedef": {}, "type": "message"},
                    "26": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "6": {"type": "string"},
            "7": {"type": "int"},
            "9": {
                "field_order": ["1", "2", "3", "4", "7", "8", "9", "11"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                    "7": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                    "11": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                },
                "type": "message",
            },
            "11": {"seen_repeated": True, "type": "int"},
            "12": {
                "field_order": ["2", "3", "4"],
                "message_typedef": {
                    "2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "13": {"message_typedef": {}, "type": "message"},
            "15": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"},
            "18": {
                "field_order": ["169945741"],
                "message_typedef": {
                    "169945741": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "19": {
                "field_order": ["1", "2", "3", "5", "6", "7", "8"],
                "message_typedef": {
                    "1": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "2": {"field_order": ["1", "1", "1", "1", "1", "1"], "message_typedef": {"1": {"seen_repeated": True, "type": "int"}}, "type": "message"},
                    "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "5": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "7": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                },
                "type": "message",
            },
            "20": {
                "field_order": ["1", "2", "3"],
                "message_typedef": {
                    "1": {"type": "int"},
                    "2": {"type": "string"},
                    "3": {
                        "field_order": ["1", "2"],
                        "message_typedef": {
                            "1": {"type": "string"},
                            "2": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                },
                "type": "message",
            },
            "21": {
                "field_order": ["2", "3", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "16"],
                "message_typedef": {
                    "2": {
                        "field_order": ["2", "4", "5"],
                        "message_typedef": {
                            "2": {"field_order": ["4"], "message_typedef": {"4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "3": {
                        "field_order": ["2", "4"],
                        "message_typedef": {
                            "2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "7": {"field_order": ["1", "2", "3"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}, "3": {"type": "string"}}, "type": "message"},
                    "8": {
                        "field_order": ["3", "4", "5"],
                        "message_typedef": {
                            "3": {
                                "field_order": ["1", "3"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1"],
                                        "message_typedef": {
                                            "1": {
                                                "field_order": ["2", "4"],
                                                "message_typedef": {
                                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
                                                    "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                },
                                                "type": "message",
                                            }
                                        },
                                        "type": "message",
                                    },
                                    "3": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "5": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["2", "4"],
                                        "message_typedef": {
                                            "2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
                                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "9": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "10": {
                        "field_order": ["1", "3", "5", "6", "7", "9", "10"],
                        "message_typedef": {
                            "1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "9": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "11": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                    "14": {"message_typedef": {}, "type": "message"},
                    "16": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                },
                "type": "message",
            },
            "22": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"},
            "25": {
                "field_order": ["1", "2"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "26": {"message_typedef": {}, "type": "message"},
        },
        "type": "message",
    },
    "2": {
        "field_order": ["1", "2"],
        "message_typedef": {
            "1": {
                "field_order": ["1"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}},
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "2": {"message_typedef": {}, "type": "message"},
        },
        "type": "message",
    },
}

GET_LIB_PAGE = {
    "1": {
        "field_order": ["1", "2", "3", "4", "6", "7", "9", "11", "11", "12", "13", "15", "18", "19", "20", "21", "22", "25"],
        "message_typedef": {
            "1": {
                "field_order": ["1", "5", "8", "9", "11", "12", "14", "15", "17", "19", "22", "23"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "3", "4", "5", "6", "7", "15", "16", "17", "19", "20", "21", "25", "30", "31", "32", "33", "34", "36", "37", "38", "39", "40", "41"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "6": {"message_typedef": {}, "type": "message"},
                            "7": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"message_typedef": {}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {
                                "field_order": ["5", "6"],
                                "message_typedef": {"5": {"field_order": ["3"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "6": {"message_typedef": {}, "type": "message"}},
                                "type": "message",
                            },
                            "25": {"message_typedef": {}, "type": "message"},
                            "30": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "33": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "36": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "40": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "5": {
                        "field_order": ["2", "3", "4", "5"],
                        "message_typedef": {
                            "2": {
                                "field_order": ["2", "4", "5", "6"],
                                "message_typedef": {
                                    "2": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    },
                                    "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "6": {"type": "int"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                            "5": {
                                "field_order": ["1", "3"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["2", "3"],
                                        "message_typedef": {
                                            "2": {"field_order": ["3", "4"], "message_typedef": {"3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                            "3": {
                                                "field_order": ["2", "3"],
                                                "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}},
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "3": {"type": "int"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "8": {"message_typedef": {}, "type": "message"},
                    "9": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "4": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["3", "4"],
                                        "message_typedef": {
                                            "3": {
                                                "field_order": ["1"],
                                                "message_typedef": {
                                                    "1": {
                                                        "field_order": ["1", "2", "3"],
                                                        "message_typedef": {
                                                            "1": {
                                                                "field_order": ["5", "6"],
                                                                "message_typedef": {
                                                                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                    "6": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                            "2": {"message_typedef": {}, "type": "message"},
                                                            "3": {
                                                                "field_order": ["1", "2"],
                                                                "message_typedef": {
                                                                    "1": {
                                                                        "field_order": ["5", "6"],
                                                                        "message_typedef": {
                                                                            "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                                                            "6": {"message_typedef": {}, "type": "message"},
                                                                        },
                                                                        "type": "message",
                                                                    },
                                                                    "2": {"message_typedef": {}, "type": "message"},
                                                                },
                                                                "type": "message",
                                                            },
                                                        },
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "4": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "11": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "12": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "15": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "17": {"field_order": ["1", "4"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "19": {
                        "field_order": ["2", "3", "4"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "22": {"message_typedef": {}, "type": "message"},
                    "23": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "2": {
                "field_order": ["1", "4", "9", "11", "14", "17", "18", "20", "23"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2", "3", "4", "5", "6", "7", "8", "10", "12", "13", "15", "18"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "4": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {
                                "field_order": ["1", "2", "3", "4", "5", "7"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"message_typedef": {}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "7": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                            "12": {"message_typedef": {}, "type": "message"},
                            "13": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "15": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                    "11": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "4", "5", "6", "9"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                    "5": {"message_typedef": {}, "type": "message"},
                                    "6": {"message_typedef": {}, "type": "message"},
                                    "9": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "14": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1", "2"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1", "2", "3"],
                                        "message_typedef": {
                                            "1": {"message_typedef": {}, "type": "message"},
                                            "2": {
                                                "field_order": ["2"],
                                                "message_typedef": {
                                                    "2": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    }
                                                },
                                                "type": "message",
                                            },
                                            "3": {
                                                "field_order": ["4", "5"],
                                                "message_typedef": {
                                                    "4": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                    "5": {
                                                        "field_order": ["1", "3"],
                                                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}},
                                                        "type": "message",
                                                    },
                                                },
                                                "type": "message",
                                            },
                                        },
                                        "type": "message",
                                    },
                                    "2": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    },
                    "17": {"message_typedef": {}, "type": "message"},
                    "18": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "20": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "23": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "3": {
                "field_order": ["2", "3", "4", "7", "12", "13", "14", "15", "16", "18", "19", "20", "24", "25"],
                "message_typedef": {
                    "2": {"message_typedef": {}, "type": "message"},
                    "3": {
                        "field_order": ["2", "3", "7", "8", "14", "16", "17", "18", "19", "20", "21", "22", "23", "27", "29", "30", "31", "32", "34", "37", "38", "39", "41"],
                        "message_typedef": {
                            "2": {"message_typedef": {}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                            "14": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "16": {"message_typedef": {}, "type": "message"},
                            "17": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "18": {"message_typedef": {}, "type": "message"},
                            "19": {"message_typedef": {}, "type": "message"},
                            "20": {"message_typedef": {}, "type": "message"},
                            "21": {"message_typedef": {}, "type": "message"},
                            "22": {"message_typedef": {}, "type": "message"},
                            "23": {"message_typedef": {}, "type": "message"},
                            "27": {
                                "field_order": ["1", "2"],
                                "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                                "type": "message",
                            },
                            "29": {"message_typedef": {}, "type": "message"},
                            "30": {"message_typedef": {}, "type": "message"},
                            "31": {"message_typedef": {}, "type": "message"},
                            "32": {"message_typedef": {}, "type": "message"},
                            "34": {"message_typedef": {}, "type": "message"},
                            "37": {"message_typedef": {}, "type": "message"},
                            "38": {"message_typedef": {}, "type": "message"},
                            "39": {"message_typedef": {}, "type": "message"},
                            "41": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "4": {"field_order": ["2", "3", "4"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "7": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                    "14": {
                        "field_order": ["1", "2", "3"],
                        "message_typedef": {
                            "1": {"message_typedef": {}, "type": "message"},
                            "2": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                },
                                "type": "message",
                            },
                            "3": {
                                "field_order": ["1", "2", "3", "4"],
                                "message_typedef": {
                                    "1": {"message_typedef": {}, "type": "message"},
                                    "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                                    "3": {"message_typedef": {}, "type": "message"},
                                    "4": {"message_typedef": {}, "type": "message"},
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                    "15": {"message_typedef": {}, "type": "message"},
                    "16": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "18": {"message_typedef": {}, "type": "message"},
                    "19": {
                        "field_order": ["4", "6", "7", "8"],
                        "message_typedef": {
                            "4": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "6": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"field_order": ["2", "3"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "8": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "20": {"message_typedef": {}, "type": "message"},
                    "24": {"message_typedef": {}, "type": "message"},
                    "25": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "4": {"type": "string"},
            "6": {"type": "string"},
            "7": {"type": "int"},
            "9": {
                "field_order": ["1", "2", "3", "4", "7", "8", "9"],
                "message_typedef": {
                    "1": {
                        "field_order": ["2"],
                        "message_typedef": {"2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["2"], "message_typedef": {"2": {"type": "int"}}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                    "7": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"},
                    "9": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "11": {"seen_repeated": True, "type": "int"},
            "12": {
                "field_order": ["2", "3", "4"],
                "message_typedef": {
                    "2": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "4": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "13": {"message_typedef": {}, "type": "message"},
            "15": {"field_order": ["3"], "message_typedef": {"3": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"},
            "18": {
                "field_order": ["169945741"],
                "message_typedef": {
                    "169945741": {
                        "field_order": ["1"],
                        "message_typedef": {
                            "1": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            }
                        },
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "19": {
                "field_order": ["1", "2", "3", "5", "6", "7", "8"],
                "message_typedef": {
                    "1": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "2": {"field_order": ["1", "1", "1", "1", "1", "1"], "message_typedef": {"1": {"seen_repeated": True, "type": "int"}}, "type": "message"},
                    "3": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "5": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "7": {"field_order": ["1", "2"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "8": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                },
                "type": "message",
            },
            "20": {
                "field_order": ["1", "2", "3"],
                "message_typedef": {
                    "1": {"type": "int"},
                    "2": {"type": "string"},
                    "3": {
                        "field_order": ["1", "2"],
                        "message_typedef": {
                            "1": {"type": "string"},
                            "2": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "4", "5", "6", "7", "8", "11", "12", "13", "15", "16", "17", "18"],
                                        "message_typedef": {
                                            "4": {"seen_repeated": True, "type": "int"},
                                            "5": {"type": "int"},
                                            "6": {"type": "int"},
                                            "7": {"type": "int"},
                                            "8": {"type": "int"},
                                            "11": {"type": "int"},
                                            "12": {"type": "int"},
                                            "13": {"type": "int"},
                                            "15": {"type": "int"},
                                            "16": {"type": "int"},
                                            "17": {"type": "int"},
                                            "18": {"type": "int"},
                                        },
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                        },
                        "type": "message",
                    },
                },
                "type": "message",
            },
            "21": {
                "field_order": ["2", "3", "5", "6", "7", "8", "9", "10", "11", "12", "13"],
                "message_typedef": {
                    "2": {"field_order": ["2", "4", "5"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "4": {"message_typedef": {}, "type": "message"}, "5": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "3": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"},
                    "5": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "6": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "7": {"field_order": ["1", "2", "3"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}, "3": {"type": "string"}}, "type": "message"},
                    "8": {
                        "field_order": ["3", "4"],
                        "message_typedef": {
                            "3": {
                                "field_order": ["1"],
                                "message_typedef": {
                                    "1": {
                                        "field_order": ["1"],
                                        "message_typedef": {"1": {"field_order": ["2"], "message_typedef": {"2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"}},
                                        "type": "message",
                                    }
                                },
                                "type": "message",
                            },
                            "4": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "9": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                    "10": {
                        "field_order": ["1", "3", "5", "6", "7", "9", "10"],
                        "message_typedef": {
                            "1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "3": {"message_typedef": {}, "type": "message"},
                            "5": {"message_typedef": {}, "type": "message"},
                            "6": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"},
                            "7": {"message_typedef": {}, "type": "message"},
                            "9": {"message_typedef": {}, "type": "message"},
                            "10": {"message_typedef": {}, "type": "message"},
                        },
                        "type": "message",
                    },
                    "11": {"message_typedef": {}, "type": "message"},
                    "12": {"message_typedef": {}, "type": "message"},
                    "13": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
            "22": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
            "25": {
                "field_order": ["1", "2"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"}},
                        "type": "message",
                    },
                    "2": {"message_typedef": {}, "type": "message"},
                },
                "type": "message",
            },
        },
        "type": "message",
    },
    "2": {
        "field_order": ["1", "2"],
        "message_typedef": {
            "1": {
                "field_order": ["1"],
                "message_typedef": {
                    "1": {
                        "field_order": ["1", "2"],
                        "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}, "2": {"message_typedef": {}, "type": "message"}},
                        "type": "message",
                    }
                },
                "type": "message",
            },
            "2": {"message_typedef": {}, "type": "message"},
        },
        "type": "message",
    },
}

SET_CAPTION = {"2": {"type": "string"}, "3": {"type": "string"}}

SET_ARCHIVED = {"1": {"seen_repeated": True, "field_order": ["1", "2"], "message_typedef": {"1": {"type": "string"}, "2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"}}, "type": "message"}, "3": {"type": "int"}}

SET_FAVORITE = {
    "1": {"field_order": ["2"], "message_typedef": {"2": {"type": "string"}}, "type": "message"},
    "2": {"field_order": ["1"], "message_typedef": {"1": {"type": "int"}}, "type": "message"},
    "3": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["19"], "message_typedef": {"19": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
}

GET_DOWNLOAD_URLS = {
    "1": {"field_order": ["1"], "message_typedef": {"1": {"field_order": ["1"], "message_typedef": {"1": {"type": "string"}}, "type": "message"}}, "type": "message"},
    "2": {
        "field_order": ["1", "5"],
        "message_typedef": {
            "1": {"field_order": ["7"], "message_typedef": {"7": {"field_order": ["2"], "message_typedef": {"2": {"message_typedef": {}, "type": "message"}}, "type": "message"}}, "type": "message"},
            "5": {
                "field_order": ["2", "3", "5"],
                "message_typedef": {
                    "2": {"message_typedef": {}, "type": "message"},
                    "3": {"message_typedef": {}, "type": "message"},
                    "5": {"field_order": ["1", "3"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}, "3": {"type": "int"}}, "type": "message"},
                },
                "type": "message",
            },
        },
        "type": "message",
    },
}


RESTORE_FROM_TRASH = {
    "2": {"type": "int"},
    "3": {"type": "string"},
    "4": {"type": "int"},
    "8": {
        "field_order": ["4"],
        "message_typedef": {
            "4": {
                "field_order": ["2", "3"],
                "message_typedef": {"2": {"message_typedef": {}, "type": "message"}, "3": {"field_order": ["1"], "message_typedef": {"1": {"message_typedef": {}, "type": "message"}}, "type": "message"}},
                "type": "message",
            }
        },
        "type": "message",
    },
    "9": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"field_order": ["1", "2"], "message_typedef": {"1": {"type": "int"}, "2": {"type": "string"}}, "type": "message"}}, "type": "message"},
}

LIB_STATE_RESPONSE_FIX = {
    "1": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"2": {"type": "message", "message_typedef": {"4": {"type": "string"}}}}}}},
}
