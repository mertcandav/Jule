#include <iostream>
#include <windows.h>

int main() {
  HANDLE hConsole = GetStdHandle(STD_OUTPUT_HANDLE);
  wchar_t text[] = {104, 101, 108, 108, 111, 32, 119, 111, 114, 108, 100, 0};
  WriteConsoleW(hConsole, text, 11, nullptr, nullptr);
  return 0;
}
