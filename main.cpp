#include <iostream>
#include <windows.h>

int main() {
  HANDLE hConsole = GetStdHandle(STD_OUTPUT_HANDLE);
  wchar_t text[] = L"Merhaba Dünya!";
  std::wcout << text << std::endl;
  WriteConsoleW(hConsole, text, _countof(text), nullptr, nullptr);
  return 0;
}
