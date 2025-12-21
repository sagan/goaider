# goaider

自用的 CLI 工具 N 合一大杂烩。使用 Go 开发。主要包括 AIGC 应用相关的一些工具；也包括个人日常用到的一些其他小工具。

## 主要命令

使用 `goaider <command> -h` 查看各个命令的帮助。部分命令需要通过环境变量(例如 GEMINI_API_KEY 等)配置外部 API 的认证信息。

- `goaider chat` : 和 LLM 聊天。支持输入文件作为 prompt。支持 interactive shell 模式。支持 Gemini, OpenAI, OpenRouter, 任意 OpenAI API 兼容的 LLM。
- `goaider caption` : 使用 LLM 生成目录里所有图片文件的 caption 文件 (.txt)。用于图片模型 LoRa 微调准备数据集。
- `goaider copy` : 复制 stdin 到剪贴板。仅支持 Windows。
- `goaider crop` : 自动裁剪并缩放目录里所有图片到 1024x1024 像素。用于图片模型 LoRa 微调准备数据集。
- `goaider csv` : CSV 文件常用的各种操作，包括 uniq (去重)、sort (排序)、join (关联查询)、query (使用 SQL 查询 CSV)、exec (对 CSV 里的每一行执行一个指定命令行)、txt2csv (将多个 txt 文件合并为 CSV, 每个 txt 文件作为一列)、excel2csv (将 Excel 文件转换为 CSV)等。
- `goaider extractall` : 一键解压目录里所有压缩包类型文件(rar / 7z / zip 等)。支持自动识别 zip 文件名编码；支持各种类型的分卷压缩包格式 (.zip + z01 + z02; .part1.exe + .part2.rar; .7z.001 + .7z.002 等等)；支持对加密压缩包用多个密码尝试解密。
- `goaider indexfiles` : 索引(递归)目录里所有指定类型文件的元信息(文件名、大小、sha256等)到 csv 文件。支持索引媒体文件的元信息；支持读取指定后缀的元信息文件 (例如 `<filename>.txt` 或 `<filename>.wav.json`)里的数据并保存到生成的 CSV 里。适用于准备 AIGC 的数据集信息。
- `goaider mediainfo` : 显示媒体文件元信息。默认仅支持图片文件；如果安装了 ffprobe ，也支持视频和音频文件。
- `goaider parsetfef` : 解析 TensorFlow event 文件 (`events.out.tfevents.*`)，生成 csv 或人类可读的文件。用于分析模型训练效果。
- `goaider paste` : 将剪贴板里内容保存为文件。仅支持 Windows。
- `goaider base64encode` / `goaider base64decode` : base64 编码 / 解码。
- `goaider rand` / `goaider randb` / `goaider randu`: 生成一个密码学安全的随机字符串 / 随机二进制 bytes / 随机 uuid。
- `goaider stt` (speech to text) : 使用 LLM 生成目录里所有音频文件的文本转写(transcript)。适用于 TTS 模型训练准备数据集。
- `goaider translate` : 使用 Google Cloud Translation API 翻译文本。支持翻译文件；支持 interactive shell 模式(输入原文；输出译文)；支持自动将译文复制到剪贴板(仅限 Windows)。设计用途是将中文 prompt 翻译为英文然后调用图片生成模型。
- `goaider tts` : 将文本转换为语音 (Text to speech) 并播放。仅支持 Windows。
- `goaider play <foo.wav>` : 播放音频文件。仅支持 Windows。
- `goaider comfyui` : ComfyUI 相关的功能。
  - `goaider comfyui run <workflow.json>` : 直接运行 json / png 格式的 workflow 并保存输出文件。
  - `goaider comfyui batchgen` : 批量运行 AIGC 图像生成任务。通过 csv 文件读取输入作为 prompt。
  - `goaider comfyui batchi2v` : 批量运行 image-to-video 视频生成任务。读取输入目录下所有图片文件，使用 LLM 生成提示词，然后生成视频。
  - `goaider comfyui parsemeta <input.png>` : 从 ComfyUI 生成的 PNG 图片里提取元数据：即生成该图片时使用的工作流(workflow)和提示(prompt)信息。

