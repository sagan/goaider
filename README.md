# goaider

自用的 CLI 工具 N 合一大杂烩。使用 Go 开发。主要包括 AIGC 应用相关的一些工具；也包括个人日常用到的一些其他小工具。

## 主要命令

使用 `goaider <command> -h` 查看各个命令的帮助。部分命令需要通过环境变量(例如 GEMINI_API_KEY 等)配置外部 API 的认证信息。

- `goaider caption` : 使用 Gemini 生成目录里所有图片文件的 caption 文件 (.txt)。用于图片模型 LoRa 微调准备数据集。
- `goaider copy` : 复制 stdin 到剪贴板。仅支持 Windows。
- `goaider crop` : 自动裁剪并缩放目录里所有图片到 1024x1024 像素。用于图片模型 LoRa 微调准备数据集。
- `goaider csv` : CSV 文件常用的各种操作，包括 uniq (去重)、sort (排序)、join (关联查询)、query (使用 SQL 查询 CSV)、exec (对 CSV 里的每一行执行一个指定命令行)等。
- `goaider extractall` : 一键解压目录里所有压缩包类型文件(rar / 7z / zip 等)。支持自动识别 zip 文件名编码；支持各种类型的分卷压缩包格式 (.zip + z01 + z02; .part1.exe + .part2.rar; .7z.001 + .7z.002 等等)；支持对加密压缩包用多个密码尝试解密。
- `goaider indexfiles` : 索引(递归)目录里所有指定类型文件的元信息(文件名、大小、sha256等)到 csv 文件。支持读取指定后缀的元信息文件 (例如 `<filename>.txt` 或 `<filename>.wav.json`)里的数据并保存到生成的 CSV 里。适用于准备 AIGC 的数据集信息。
- `goaider parsetfef` : 解析 TensorFlow event 文件 (`events.out.tfevents.*`)，生成 csv 或人类可读的文件。用于分析模型训练效果。
- `goaider paste` : 将剪贴板里内容保存为文件。仅支持 Windows。
- `goaider base64encode` / `goaider base64decode` : base64 编码 / 解码。
- `goaider rand` / `goaider randb` : 生成一个密码学安全的随机字符串 / 随机二进制 bytes。
- `goaider stt` (speech to text) : 使用 Gemini 生成目录里所有音频文件的文本转写(transcript)。适用于 TTS 模型训练准备数据集。
- `goaider translate` : 使用 Google Cloud Translation API 翻译文本。支持 interactive shell 模式(输入原文；输出译文)；支持自动将译文复制到剪贴板(仅限 Windows)。设计用途是将中文 prompt 翻译为英文然后调用图片生成模型。。
- `goaider comfyui` : ComfyUI 相关的功能。例如直接运行 json / png 格式的 workflow 并保存输出文件。

