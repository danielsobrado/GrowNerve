#include "command_replay.h"

#include <cstring>

namespace {
constexpr const char *kNamespace = "grownerve";
constexpr const char *kRecordKey = "cmdreplay";
constexpr uint32_t kStorageMagic = 0x474E4352;  // GNCR
constexpr uint16_t kStorageVersion = 1;

struct PersistedReplayCache {
  uint32_t magic = kStorageMagic;
  uint16_t storageVersion = kStorageVersion;
  uint8_t nextIndex = 0;
  uint8_t reserved = 0;
  struct Entry {
    char commandId[37] = {0};
    char result[16] = {0};
    char reasonCode[65] = {0};
  } entries[8]{};
};

bool validText(const char *value, size_t maximumLength) {
  return value != nullptr && strnlen(value, maximumLength + 1) <= maximumLength;
}
}  // namespace

bool CommandReplayCache::load(Preferences &storage) {
  if (!storage.begin(kNamespace, true)) return false;
  const size_t stored = storage.getBytesLength(kRecordKey);
  if (stored != sizeof(PersistedReplayCache)) {
    storage.end();
    return false;
  }

  PersistedReplayCache record{};
  const size_t read = storage.getBytes(kRecordKey, &record, sizeof(record));
  storage.end();
  if (read != sizeof(record) || record.magic != kStorageMagic || record.storageVersion != kStorageVersion ||
      record.nextIndex >= kCapacity) {
    return false;
  }

  for (size_t index = 0; index < kCapacity; ++index) {
    record.entries[index].commandId[kCommandIdLength] = '\0';
    record.entries[index].result[kResultLength] = '\0';
    record.entries[index].reasonCode[kReasonCodeLength] = '\0';
    strncpy(entries_[index].commandId, record.entries[index].commandId, sizeof(entries_[index].commandId) - 1);
    strncpy(entries_[index].result, record.entries[index].result, sizeof(entries_[index].result) - 1);
    strncpy(entries_[index].reasonCode, record.entries[index].reasonCode, sizeof(entries_[index].reasonCode) - 1);
  }
  nextIndex_ = record.nextIndex;
  return true;
}

bool CommandReplayCache::find(const char *commandId, CommandReplayResult &out) const {
  if (commandId == nullptr || strlen(commandId) != kCommandIdLength) return false;
  for (const Entry &entry : entries_) {
    if (entry.commandId[0] == '\0' || strcmp(entry.commandId, commandId) != 0) continue;
    out.result = entry.result;
    out.reasonCode = entry.reasonCode;
    return true;
  }
  return false;
}

bool CommandReplayCache::remember(Preferences &storage, const char *commandId, const char *result,
                                  const char *reasonCode) {
  if (commandId == nullptr || strlen(commandId) != kCommandIdLength || !validText(result, kResultLength) ||
      !validText(reasonCode, kReasonCodeLength)) {
    return false;
  }

  size_t target = kCapacity;
  for (size_t index = 0; index < kCapacity; ++index) {
    if (strcmp(entries_[index].commandId, commandId) == 0) {
      target = index;
      break;
    }
  }
  if (target == kCapacity) {
    target = nextIndex_;
    nextIndex_ = static_cast<uint8_t>((nextIndex_ + 1) % kCapacity);
  }

  Entry &entry = entries_[target];
  memset(&entry, 0, sizeof(entry));
  strncpy(entry.commandId, commandId, sizeof(entry.commandId) - 1);
  strncpy(entry.result, result, sizeof(entry.result) - 1);
  strncpy(entry.reasonCode, reasonCode, sizeof(entry.reasonCode) - 1);

  PersistedReplayCache record{};
  record.nextIndex = nextIndex_;
  for (size_t index = 0; index < kCapacity; ++index) {
    strncpy(record.entries[index].commandId, entries_[index].commandId, sizeof(record.entries[index].commandId) - 1);
    strncpy(record.entries[index].result, entries_[index].result, sizeof(record.entries[index].result) - 1);
    strncpy(record.entries[index].reasonCode, entries_[index].reasonCode, sizeof(record.entries[index].reasonCode) - 1);
  }

  if (!storage.begin(kNamespace, false)) return false;
  const size_t written = storage.putBytes(kRecordKey, &record, sizeof(record));
  storage.end();
  return written == sizeof(record);
}
