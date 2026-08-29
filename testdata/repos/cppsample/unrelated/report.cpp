#include <string>

namespace reporting {
std::string renderReport(int value) {
    return "report:" + std::to_string(value);
}
}

