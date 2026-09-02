#pragma once

#include <Arduino.h>
#include <Preferences.h>

struct CommandReplayResult {
  String result;
  String reasonCode;
};

class CommandReplayCache {
 public:
  bool load(Preferences &storage);
  bool find(const char *commandId, CommandReplayResult &out) const;
  bool remember(Preferences &storage, const char *commandId, const char *result, const char *reasonCode);

 private:
  static constexpr size_t kCapacity = 8;
  static constexpr size_t kCommandIdLength = 36;
  static constexpr size_t kResultLength = 15;
  static constexpr size_t kReasonCodeLength = 64;

  struct Entry {
    char commandId[kCommandIdLength + 1] = {0};
    char result[kResultLength + 1] = {0};
    char reasonCode[kReasonCodeLength + 1] = {0};
  };

  Entry entries_[kCapacity]{};
  uint8_t nextIndex_ = 0;
};
