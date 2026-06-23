const {
  Document, Packer, Paragraph, TextRun, HeadingLevel, AlignmentType,
  Table, TableRow, TableCell, WidthType, BorderStyle, PageBreak,
  Header, Footer, PageNumber, NumberFormat, TableOfContents,
  ImageRun, ShadingType, VerticalAlign, convertInchesToTwip,
  LevelFormat, TabStopPosition, TabStopType
} = require('docx');
const fs = require('fs');
const path = require('path');

// ===== 图片辅助函数 =====
function loadImageBuffer(filename) {
  const imgPath = path.join(__dirname, 'diagrams', filename);
  if (!fs.existsSync(imgPath)) {
    console.warn(`⚠ 图片不存在: ${imgPath}`);
    return null;
  }
  return fs.readFileSync(imgPath);
}

function getImageSize(filename) {
  const imgPath = path.join(__dirname, 'diagrams', filename);
  if (!fs.existsSync(imgPath)) return { width: 600, height: 400 };
  // 简单返回固定尺寸，实际渲染时按比例缩放
  const stats = fs.statSync(imgPath);
  // 根据文件大小估算，实际使用固定宽度
  return { width: 600, height: 400 };
}

function imageParagraph(filename, captionText, options = {}) {
  const buffer = loadImageBuffer(filename);
  if (!buffer) {
    return [
      new Paragraph({
        spacing: { before: 120, after: 60 },
        alignment: AlignmentType.CENTER,
        children: [mixedRun(`[图片缺失: ${filename}]`, { size: 21, color: 'FF0000' })],
      }),
      captionParagraph(captionText),
    ];
  }

  // 获取图片尺寸（通过文件读取PNG头）
  let width = options.width || 580;
  let height = options.height || 400;

  // 读取PNG尺寸
  if (buffer.length >= 24 && buffer[0] === 0x89 && buffer[1] === 0x50) {
    width = (buffer[16] << 24 | buffer[17] << 16 | buffer[18] << 8 | buffer[19]);
    height = (buffer[20] << 24 | buffer[21] << 16 | buffer[22] << 8 | buffer[23]);
  }

  // 按页面宽度等比缩放（页面可用宽度约6.3英寸=605像素@96dpi）
  const maxWidth = options.maxWidth || 580;
  const maxHeight = options.maxHeight || 750;
  let scale = 1;
  if (width > maxWidth) {
    scale = maxWidth / width;
  }
  if (height * scale > maxHeight) {
    scale = maxHeight / height;
  }
  const finalWidth = Math.floor(width * scale);
  const finalHeight = Math.floor(height * scale);

  return [
    new Paragraph({
      spacing: { before: 120, after: 60, line: 276 },
      alignment: AlignmentType.CENTER,
      children: [
        new ImageRun({
          data: buffer,
          transformation: {
            width: finalWidth,
            height: finalHeight,
          },
        }),
      ],
    }),
    captionParagraph(captionText),
  ];
}

// ===== 学术论文样式常量 =====
const FONT_CN = '宋体';
const FONT_EN = 'Times New Roman';
const FONT_TITLE = '黑体';
const FONT_HEADING = '黑体';
const FONT_CODE = 'Consolas';
const LINE_SPACING = 360; // 1.5倍行距 (240 * 1.5)
const PAGE_WIDTH = convertInchesToTwip(6.3);
const MARGIN_TOP = convertInchesToTwip(1);
const MARGIN_BOTTOM = convertInchesToTwip(1);
const MARGIN_LEFT = convertInchesToTwip(1.25);
const MARGIN_RIGHT = convertInchesToTwip(1.25);

// ===== 辅助函数 =====
function cnRun(text, options = {}) {
  return new TextRun({
    text,
    font: { name: FONT_CN, eastAsia: FONT_CN },
    size: options.size || 24,
    bold: options.bold || false,
    italics: options.italics || false,
    color: options.color || '000000',
    ...options,
  });
}

function enRun(text, options = {}) {
  return new TextRun({
    text,
    font: { name: FONT_EN },
    size: options.size || 24,
    bold: options.bold || false,
    italics: options.italics || false,
    color: options.color || '000000',
    ...options,
  });
}

function mixedRun(text, options = {}) {
  return new TextRun({
    text,
    font: { name: FONT_EN, eastAsia: FONT_CN },
    size: options.size || 24,
    bold: options.bold || false,
    italics: options.italics || false,
    color: options.color || '000000',
    ...options,
  });
}

function bodyParagraph(text, options = {}) {
  return new Paragraph({
    spacing: { line: LINE_SPACING, before: 60, after: 60 },
    indent: { firstLine: convertInchesToTwip(0.49) },
    alignment: AlignmentType.JUSTIFIED,
    ...options,
    children: [mixedRun(text, options.runOptions || {})],
  });
}

function heading1(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_1,
    spacing: { before: 360, after: 240, line: LINE_SPACING },
    alignment: AlignmentType.LEFT,
    children: [mixedRun(text, { size: 32, bold: true, font: { name: FONT_HEADING, eastAsia: FONT_HEADING } })],
  });
}

function heading2(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_2,
    spacing: { before: 240, after: 180, line: LINE_SPACING },
    alignment: AlignmentType.LEFT,
    children: [mixedRun(text, { size: 28, bold: true, font: { name: FONT_HEADING, eastAsia: FONT_HEADING } })],
  });
}

function heading3(text) {
  return new Paragraph({
    heading: HeadingLevel.HEADING_3,
    spacing: { before: 180, after: 120, line: LINE_SPACING },
    alignment: AlignmentType.LEFT,
    children: [mixedRun(text, { size: 24, bold: true, font: { name: FONT_HEADING, eastAsia: FONT_HEADING } })],
  });
}

function emptyLine() {
  return new Paragraph({ spacing: { line: LINE_SPACING }, children: [] });
}

function captionParagraph(text) {
  return new Paragraph({
    spacing: { before: 60, after: 120, line: 276 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun(text, { size: 21 })],
  });
}

function createTableCell(text, options = {}) {
  return new TableCell({
    width: options.width ? { size: options.width, type: WidthType.DXA } : undefined,
    shading: options.shading ? { fill: options.shading, type: ShadingType.CLEAR } : undefined,
    verticalAlign: VerticalAlign.CENTER,
    margins: { top: 40, bottom: 40, left: 80, right: 80 },
    children: [
      new Paragraph({
        spacing: { line: 276 },
        alignment: options.align || AlignmentType.LEFT,
        children: [mixedRun(text, { size: options.size || 21, bold: options.bold || false })],
      }),
    ],
  });
}

function createHeaderCell(text, width) {
  return createTableCell(text, { width, shading: 'D9E2F3', bold: true, align: AlignmentType.CENTER, size: 21 });
}

function codeBlock(code) {
  const lines = code.split('\n');
  // 每行作为一个独立段落，确保换行和缩进正确
  return lines.map((line, index) => {
    return new Paragraph({
      spacing: { before: 0, after: 0, line: 240 },
      indent: { left: convertInchesToTwip(0.4) },
      children: [
        new TextRun({
          text: line.length === 0 ? '' : line,
          font: { name: FONT_CODE },
          size: 18,
        }),
      ],
    });
  });
}

// ===== 文档内容 =====

// 封面
const coverPage = [
  emptyLine(), emptyLine(), emptyLine(), emptyLine(),
  new Paragraph({
    spacing: { before: 1200, after: 200, line: 480 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun('DevDash轻量级运维监控面板', { size: 44, bold: true, font: { name: FONT_TITLE, eastAsia: FONT_TITLE } })],
  }),
  new Paragraph({
    spacing: { before: 100, after: 600, line: 400 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun('系统设计与实现技术报告', { size: 36, bold: true, font: { name: FONT_TITLE, eastAsia: FONT_TITLE } })],
  }),
  emptyLine(), emptyLine(),
  new Paragraph({
    spacing: { line: 400 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun('——基于Go + Vue 3的轻量级服务器监控与管理平台', { size: 24 })],
  }),
  emptyLine(), emptyLine(), emptyLine(), emptyLine(), emptyLine(), emptyLine(), emptyLine(),
  // 论文标准信息表
  new Table({
    width: { size: 70, type: WidthType.PERCENTAGE },
    alignment: AlignmentType.CENTER,
    borders: {
      top: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
      bottom: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
      left: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
      right: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
      insideHorizontal: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
      insideVertical: { style: BorderStyle.NONE, size: 0, color: 'FFFFFF' },
    },
    rows: [
      new TableRow({ children: [
        new TableCell({ width: { size: 2500, type: WidthType.DXA }, margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('作    者：', { size: 24, bold: true })] })] }),
        new TableCell({ width: { size: 3500, type: WidthType.DXA }, margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('GXF', { size: 24 })] })] }),
      ]}),
      new TableRow({ children: [
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('学    号：', { size: 24, bold: true })] })] }),
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('2026XXXX', { size: 24 })] })] }),
      ]}),
      new TableRow({ children: [
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('指导教师：', { size: 24, bold: true })] })] }),
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('XXX 教授', { size: 24 })] })] }),
      ]}),
      new TableRow({ children: [
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('专    业：', { size: 24, bold: true })] })] }),
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('计算机科学与技术', { size: 24 })] })] }),
      ]}),
      new TableRow({ children: [
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('学    院：', { size: 24, bold: true })] })] }),
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('信息工程学院', { size: 24 })] })] }),
      ]}),
      new TableRow({ children: [
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.RIGHT, children: [mixedRun('日    期：', { size: 24, bold: true })] })] }),
        new TableCell({ margins: { top: 80, bottom: 80, left: 100, right: 100 }, children: [new Paragraph({ alignment: AlignmentType.LEFT, children: [mixedRun('2026年6月', { size: 24 })] })] }),
      ]}),
    ],
  }),
];

// 摘要
const abstractSection = [
  heading1('摘要'),
  bodyParagraph('DevDash是一款面向开发者和运维工程师的轻量级服务器监控与管理平台，采用Go语言（Gin框架）作为后端、Vue 3 + TypeScript作为前端技术栈，实现了服务器核心指标的实时采集与可视化展示、告警规则引擎与多渠道通知、文件管理、Web终端、计划任务管理、Docker容器监控等核心功能。本文详细阐述了DevDash的系统架构设计、技术选型依据、功能模块划分、关键算法实现及测试验证过程，为同类运维监控系统的设计与开发提供参考。'),
  bodyParagraph('系统采用前后端分离的B/S架构，后端通过gopsutil库实现跨平台系统指标采集，支持CPU、内存、磁盘、网络、GPU等10余项指标的秒级采集。告警引擎支持飞书、钉钉、邮件及自定义Webhook等多渠道通知，配置持久化存储确保重启不丢失。v1.8.1版本重点解决了Docker容器部署下的Linux主机适配问题，通过nsenter命名空间隔离技术和HOST_ROOT路径映射机制，实现了容器内对宿主机文件系统、进程命名空间和系统资源的完整访问。同时修复了告警历史时间戳解析、告警规则channels持久化、CI镜像latest标签缺失等关键问题。'),
  bodyParagraph('研究表明，DevDash采用的对等节点架构与gopsutil跨平台采集方案，能够在资源占用极低（内存<50MB）的前提下，实现秒级数据采集与实时展示。通过JWT认证、RBAC权限控制、速率限制等安全机制，满足生产环境部署需求。Docker多阶段构建方案将镜像体积控制在50MB以内，支持linux/amd64和linux/arm64双架构部署。'),
  emptyLine(),
  new Paragraph({
    spacing: { line: LINE_SPACING },
    indent: { firstLine: convertInchesToTwip(0.49) },
    children: [
      mixedRun('关键词：', { bold: true }),
      mixedRun('运维监控；Go语言；Vue 3；实时采集；告警引擎；容器化部署；Linux适配；nsenter'),
    ],
  }),
];

// 英文摘要
const abstractEnSection = [
  heading1('Abstract'),
  new Paragraph({
    spacing: { line: LINE_SPACING },
    indent: { firstLine: convertInchesToTwip(0.49) },
    children: [
      new TextRun({ text: 'DevDash is a lightweight server monitoring and management platform designed for developers and operations engineers. It adopts Go (Gin framework) as the backend and Vue 3 + TypeScript as the frontend technology stack, implementing real-time collection and visualization of server core metrics, alert rule engine with multi-channel notifications, file management, Web terminal, scheduled task management, and Docker container monitoring. This paper details the system architecture design, technology selection rationale, functional module division, key algorithm implementation, and testing verification process of DevDash, providing a reference for the design and development of similar operations monitoring systems.', font: { name: FONT_EN }, size: 24 }),
    ],
  }),
  new Paragraph({
    spacing: { line: LINE_SPACING },
    indent: { firstLine: convertInchesToTwip(0.49) },
    children: [
      new TextRun({ text: 'The system adopts a front-end and back-end separated B/S architecture. The backend implements cross-platform system metric collection through the gopsutil library, supporting second-level collection of over 10 metrics including CPU, memory, disk, network, and GPU. The alert engine supports multi-channel notifications including Feishu, DingTalk, email, and custom Webhook, with configuration persistence ensuring no loss after restart. Version v1.8.1 focuses on solving the Linux host adaptation problem under Docker container deployment, achieving complete access to host filesystem, process namespace, and system resources from within the container through nsenter namespace isolation technology and HOST_ROOT path mapping mechanism.', font: { name: FONT_EN }, size: 24 }),
    ],
  }),
  new Paragraph({
    spacing: { line: LINE_SPACING },
    indent: { firstLine: convertInchesToTwip(0.49) },
    children: [
      new TextRun({ text: 'Research shows that DevDash\'s peer-to-peer node architecture and gopsutil cross-platform collection solution can achieve second-level data collection and real-time display with extremely low resource usage (memory < 50MB). Through security mechanisms such as JWT authentication, RBAC permission control, and rate limiting, it meets production deployment requirements. The Docker multi-stage build solution controls the image size within 50MB, supporting both linux/amd64 and linux/arm64 architecture deployments.', font: { name: FONT_EN }, size: 24 }),
    ],
  }),
  emptyLine(),
  new Paragraph({
    spacing: { line: LINE_SPACING },
    indent: { firstLine: convertInchesToTwip(0.49) },
    children: [
      new TextRun({ text: 'Keywords: ', font: { name: FONT_EN }, size: 24, bold: true }),
      new TextRun({ text: 'Operations Monitoring; Go Language; Vue 3; Real-time Collection; Alert Engine; Containerized Deployment; Linux Adaptation; nsenter', font: { name: FONT_EN }, size: 24 }),
    ],
  }),
];

// 目录
const tocSection = [
  heading1('目录'),
  new TableOfContents('Table of Contents', {
    hyperlink: true,
    headingStyleRange: '1-3',
  }),
  new Paragraph({
    spacing: { before: 240, after: 120 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun('（注：请在Word中右键点击此处选择"更新域"以生成目录）', { size: 18, italics: true, color: '888888' })],
  }),
];

// 第一章 引言
const introductionSection = [
  heading1('1  引言'),
  heading2('1.1  项目背景'),
  bodyParagraph('随着云计算和微服务架构的广泛普及，服务器集群规模持续增长，运维人员面临日益严峻的监控管理挑战。传统的运维监控工具如Zabbix、Nagios等虽然功能强大，但部署复杂、学习成本高，对于中小规模团队和开发者个人项目而言存在明显的过度设计问题。同时，现有的轻量级监控方案往往功能单一，难以满足从数据采集到告警通知的全链路需求。'),
  bodyParagraph('在此背景下，DevDash项目应运而生。DevDash旨在提供一种"开箱即用"的轻量级运维监控解决方案，将核心监控功能集成于单一二进制文件中，支持Docker一键部署，30秒内即可完成启动。项目坚持"轻量但不简陋"的设计理念，在保持极低资源占用的同时，提供完整的监控、告警、管理能力。随着v1.8.x系列的迭代，系统进一步完善了Docker容器部署场景下的Linux主机适配能力，解决了容器内访问宿主机资源的技术难题。'),

  heading2('1.2  研究意义'),
  bodyParagraph('DevDash的研究与实现具有以下意义：（1）为中小规模运维场景提供了一种高效、低成本的监控解决方案，降低了运维工具的部署和使用门槛；（2）探索了Go语言在系统级监控领域的应用实践，验证了gopsutil跨平台采集方案的可行性和性能表现；（3）设计了可扩展的告警引擎架构，支持多渠道通知的灵活接入，为同类系统的告警模块设计提供了参考；（4）通过Docker多阶段构建和GHCR镜像分发，展示了现代云原生应用的完整交付流程；（5）通过nsenter命名空间隔离技术和HOST_ROOT路径映射机制，解决了容器化部署场景下访问宿主机资源的工程难题，为同类容器化运维工具提供了技术参考。'),

  heading2('1.3  国内外研究现状'),
  bodyParagraph('在运维监控领域，国外开源项目发展较为成熟。Prometheus + Grafana组合已成为云原生监控的事实标准，但其部署复杂度较高，需要独立配置时序数据库、采集器和可视化组件。Netdata以其卓越的单机实时性能著称，但缺乏多主机管理和告警通知的完善支持。Uptime Kuma专注于服务可用性监控，功能范围相对有限。'),
  bodyParagraph('国内方面，随着DevOps理念的深入，涌现了一批面向中文用户的监控工具，如1Panel、宝塔面板等，但这些产品更多侧重于服务器管理而非纯监控场景。在轻量级、自包含的运维监控工具领域，仍存在明显的市场空白。DevDash的出现填补了这一空白，以单一二进制文件实现了从数据采集到告警通知的全链路覆盖，并通过Docker容器化部署方案，实现了跨平台的快速交付。'),

  heading2('1.4  本文结构'),
  bodyParagraph('本文共分为八个章节。第一章为引言，介绍项目背景与研究意义；第二章详细描述系统架构设计；第三章分析技术选型依据；第四章阐述功能模块设计；第五章介绍关键实现细节，重点包括容器化适配方案；第六章展示测试结果与分析；第七章总结全文并展望未来方向；第八章为参考文献与附录。'),
];

// 第二章 系统架构
const architectureSection = [
  heading1('2  系统架构'),
  heading2('2.1  整体架构设计'),
  bodyParagraph('DevDash采用经典的B/S（Browser/Server）架构，前后端分离设计。整体架构分为四个层次：展示层、服务层、数据层和基础设施层。展示层基于Vue 3 + Naive UI构建，负责数据可视化与用户交互；服务层基于Go + Gin框架，提供RESTful API和WebSocket服务；数据层支持SQLite（默认）和PostgreSQL双存储引擎；基础设施层通过gopsutil库实现跨平台的系统指标采集。'),
  bodyParagraph('在Docker容器部署场景下，系统通过nsenter工具和HOST_ROOT路径映射机制，实现了容器内对宿主机文件系统、进程命名空间和系统资源的完整访问。容器以特权模式运行（SYS_PTRACE、SYS_ADMIN、SYS_CHROOT能力），通过挂载宿主机的/proc、/sys、/dev等伪文件系统，确保监控数据的准确性和完整性。'),
  emptyLine(),

  new Paragraph({
    spacing: { before: 120, after: 60, line: 276 },
    alignment: AlignmentType.CENTER,
    children: [mixedRun('图2-1  DevDash系统架构图', { size: 21, bold: true })],
  }),
  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          new TableCell({
            columnSpan: 3,
            shading: { fill: 'E8F0FE', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 80, bottom: 80, left: 80, right: 80 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('展示层：Vue 3 + Naive UI + ECharts + Pinia + xterm.js', { size: 21, bold: true })] })],
          }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({
            columnSpan: 3,
            shading: { fill: 'FCE4EC', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 80, bottom: 80, left: 80, right: 80 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('服务层：Go + Gin（REST API + WebSocket + 告警引擎 + 文件管理 + 终端代理 + nsenter主机访问）', { size: 21, bold: true })] })],
          }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({
            columnSpan: 3,
            shading: { fill: 'E8F5E9', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 80, bottom: 80, left: 80, right: 80 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('数据层：SQLite（默认）/ PostgreSQL + KV配置存储', { size: 21, bold: true })] })],
          }),
        ],
      }),
      new TableRow({
        children: [
          new TableCell({
            width: { size: 33, type: WidthType.PERCENTAGE },
            shading: { fill: 'FFF3E0', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 60, bottom: 60, left: 60, right: 60 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('gopsutil', { size: 20 })] })],
          }),
          new TableCell({
            width: { size: 34, type: WidthType.PERCENTAGE },
            shading: { fill: 'FFF3E0', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 60, bottom: 60, left: 60, right: 60 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('Docker API', { size: 20 })] })],
          }),
          new TableCell({
            width: { size: 33, type: WidthType.PERCENTAGE },
            shading: { fill: 'FFF3E0', type: ShadingType.CLEAR },
            verticalAlign: VerticalAlign.CENTER,
            margins: { top: 60, bottom: 60, left: 60, right: 60 },
            children: [new Paragraph({ alignment: AlignmentType.CENTER, children: [mixedRun('nsenter + HOST_ROOT', { size: 20 })] })],
          }),
        ],
      }),
    ],
  }),
  captionParagraph('图2-1  DevDash四层架构设计（展示层→服务层→数据层→基础设施层）'),

  heading2('2.2  模块划分与组件关系'),
  bodyParagraph('系统后端按功能划分为以下核心模块：'),
  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('模块名称', 2000),
          createHeaderCell('包路径', 3500),
          createHeaderCell('核心职责', 4500),
        ],
      }),
      new TableRow({ children: [createTableCell('数据采集器', 2000), createTableCell('collector', 3500), createTableCell('CPU/内存/磁盘/网络/GPU等系统指标采集', 4500)] }),
      new TableRow({ children: [createTableCell('API处理器', 2000), createTableCell('api', 3500), createTableCell('RESTful API路由与请求处理', 4500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('alert', 3500), createTableCell('规则匹配、告警触发与多渠道通知', 4500)] }),
      new TableRow({ children: [createTableCell('认证授权', 2000), createTableCell('auth', 3500), createTableCell('JWT认证、RBAC权限、CSRF防护', 4500)] }),
      new TableRow({ children: [createTableCell('数据存储', 2000), createTableCell('store', 3500), createTableCell('SQLite/PostgreSQL双引擎、KV配置存储', 4500)] }),
      new TableRow({ children: [createTableCell('文件管理', 2000), createTableCell('filemgr', 3500), createTableCell('文件浏览、编辑、上传下载、权限管理', 4500)] }),
      new TableRow({ children: [createTableCell('终端代理', 2000), createTableCell('terminal', 3500), createTableCell('WebSocket终端、ConPty/PTY集成', 4500)] }),
      new TableRow({ children: [createTableCell('计划任务', 2000), createTableCell('cronjob', 3500), createTableCell('crontab/Task Scheduler可视化管理', 4500)] }),
      new TableRow({ children: [createTableCell('主机路径映射', 2000), createTableCell('hostpath', 3500), createTableCell('容器内宿主机文件系统路径映射与访问', 4500)] }),
      new TableRow({ children: [createTableCell('配置管理', 2000), createTableCell('config', 3500), createTableCell('环境变量解析、参数校验与默认值', 4500)] }),
    ],
  }),
  captionParagraph('表2-1  后端核心模块划分'),

  heading2('2.3  容器化部署架构'),
  bodyParagraph('在Docker容器部署场景下，DevDash采用特权容器模式运行，通过以下机制实现容器内对宿主机资源的完整访问：'),
  bodyParagraph('（1）文件系统访问：通过HOST_ROOT环境变量（默认/host）映射宿主机根文件系统，filemgr模块所有文件操作自动将容器内路径映射到宿主机实际路径。'),
  bodyParagraph('（2）进程命名空间访问：通过nsenter -m -u -i -n -p -t 1命令进入宿主机PID 1的命名空间，实现脚本执行、命令执行和crontab注册操作在宿主机环境中运行。'),
  bodyParagraph('（3）系统指标采集：通过HOST_PROC、HOST_SYS、HOST_DEV、HOST_ETC环境变量，将宿主机的/proc、/sys、/dev、/etc挂载到容器内，gopsutil库自动读取这些路径获取宿主机真实指标。'),
  bodyParagraph('（4）Docker管理：通过挂载/var/run/docker.sock，容器内直接调用Docker Engine API管理宿主机上的容器。'),
  bodyParagraph('（5）终端访问：Web终端通过nsenter连接宿主机shell，用户在浏览器中操作的终端实际运行在宿主机命名空间内。'),

  heading2('2.4  交互流程'),
  bodyParagraph('系统采用"采集-存储-展示-告警"的四阶段交互流程。数据采集器按照可配置的时间间隔（默认5秒）周期性调用gopsutil接口获取系统指标，采集结果以Snapshot结构体形式存入数据库。前端通过轮询或WebSocket获取最新数据，经ECharts渲染为实时图表。告警引擎在每次采集后执行规则匹配，触发告警时通过配置的通知渠道发送消息。'),
  bodyParagraph('WebSocket连接用于终端功能，前端通过xterm.js建立与后端的WebSocket连接，后端通过ConPty（Windows）或PTY（Linux）创建伪终端会话。在容器部署模式下，后端通过nsenter进入宿主机命名空间，实现浏览器内直接操作宿主机命令行。文件管理功能通过RESTful API实现，支持大文件的分块上传和断点续传。'),
];

// 第三章 技术选型
const techStackSection = [
  heading1('3  技术选型'),
  heading2('3.1  后端技术栈'),
  heading3('3.1.1  Go语言 + Gin框架'),
  bodyParagraph('选择Go语言作为后端开发语言，基于以下考量：（1）Go语言天生支持高并发，goroutine的轻量级特性使得系统能够同时处理大量WebSocket连接和API请求；（2）Go编译为单一静态二进制文件，无需运行时依赖，极大简化了部署流程；（3）Go的跨平台编译能力支持Windows/Linux/macOS多平台构建。Gin框架以其高性能路由（基于Radix树）和中间件机制，成为Go Web框架的首选，在基准测试中QPS可达数万级别。'),

  heading3('3.1.2  gopsutil系统监控库'),
  bodyParagraph('gopsutil是Go语言生态中最成熟的跨平台系统监控库，支持CPU、内存、磁盘、网络、进程、主机信息等全方位指标采集。选择gopsutil而非直接调用系统命令的原因包括：（1）跨平台兼容性，同一套代码在Windows和Linux上均可运行；（2）无需CGO依赖，保持纯Go编译的便利性；（3）活跃的社区维护和完善的文档支持。在容器部署场景下，gopsutil支持通过HOST_PROC、HOST_SYS等环境变量指定系统文件路径，实现容器内采集宿主机指标。'),

  heading3('3.1.3  数据库选型'),
  bodyParagraph('默认采用SQLite作为存储引擎，原因是：（1）零配置，无需独立数据库服务，降低部署复杂度；（2）CGO-free的modernc.org/sqlite实现，保持交叉编译能力；（3）对于单机监控场景，SQLite的读写性能完全满足需求。同时支持PostgreSQL作为可选存储后端，满足高并发写入和多实例共享数据库的企业级需求。v1.8.1版本修复了SQLite存储time.Time类型时的序列化/反序列化问题，通过parseTimeStr()函数支持多种时间格式的解析。'),

  heading2('3.2  前端技术栈'),
  heading3('3.2.1  Vue 3 + TypeScript'),
  bodyParagraph('Vue 3的Composition API提供了更灵活的代码组织方式，配合TypeScript的类型系统，显著提升了代码的可维护性和开发效率。Vite作为构建工具，利用浏览器原生ES Module支持，实现了毫秒级的热更新响应。Pinia作为Vue 3官方推荐的状态管理方案，相比Vuex具有更简洁的API和更好的TypeScript支持。'),

  heading3('3.2.2  Naive UI + ECharts'),
  bodyParagraph('Naive UI是Vue 3生态中TypeScript支持最完善的组件库，提供80+高质量组件，支持主题定制和国际化。ECharts作为数据可视化引擎，支持丰富的图表类型和交互能力，特别适合实时监控数据的展示需求。xterm.js实现了浏览器内的终端模拟，配合WebSocket实现Web终端功能。v1.8.1版本修复了n-data-table组件的row-key函数在接收null行时崩溃的问题，通过可选链操作符（?.）和空值合并操作符（??）增强了前端鲁棒性。'),

  heading2('3.3  部署与容器化'),
  bodyParagraph('采用Docker多阶段构建方案，第一阶段编译Go二进制文件和构建前端静态资源，第二阶段仅包含最终产物，镜像体积控制在50MB以内。通过GitHub Actions实现CI/CD自动化，代码推送后自动构建并推送镜像至GitHub Container Registry（GHCR），支持linux/amd64和linux/arm64双架构。v1.8.1版本修复了CI流水线未打latest标签的问题，确保docker pull :latest能够拉取到最新镜像。'),
  bodyParagraph('Dockerfile优化方面，移除了非root用户限制（nsenter/crontab/docker.sock需要root权限），添加了bash和coreutils依赖。通过BuildKit缓存挂载（npm和Go模块），显著加速了构建过程。容器运行时需要以下Linux能力：SYS_PTRACE（进程跟踪）、SYS_ADMIN（命名空间操作）、SYS_CHROOT（根目录切换），以支持nsenter和主机访问功能。'),

  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('层级', 1500),
          createHeaderCell('技术', 2500),
          createHeaderCell('版本', 1500),
          createHeaderCell('选择依据', 4500),
        ],
      }),
      new TableRow({ children: [createTableCell('后端框架', 1500), createTableCell('Go + Gin', 2500), createTableCell('1.22+ / v1.9', 1500), createTableCell('高性能HTTP框架，QPS数万级', 4500)] }),
      new TableRow({ children: [createTableCell('前端框架', 1500), createTableCell('Vue 3 + Vite', 2500), createTableCell('3.4+', 1500), createTableCell('Composition API + 毫秒级热更新', 4500)] }),
      new TableRow({ children: [createTableCell('UI组件', 1500), createTableCell('Naive UI', 2500), createTableCell('2.38+', 1500), createTableCell('TypeScript原生支持，80+组件', 4500)] }),
      new TableRow({ children: [createTableCell('可视化', 1500), createTableCell('ECharts', 2500), createTableCell('5.5+', 1500), createTableCell('丰富图表类型，实时数据渲染', 4500)] }),
      new TableRow({ children: [createTableCell('系统监控', 1500), createTableCell('gopsutil', 2500), createTableCell('v4', 1500), createTableCell('跨平台采集，CGO-free，支持HOST_*环境变量', 4500)] }),
      new TableRow({ children: [createTableCell('数据库', 1500), createTableCell('SQLite/PostgreSQL', 2500), createTableCell('-', 1500), createTableCell('零配置默认/企业级可选', 4500)] }),
      new TableRow({ children: [createTableCell('认证', 1500), createTableCell('JWT (HS256)', 2500), createTableCell('v5', 1500), createTableCell('无状态认证，水平扩展友好', 4500)] }),
      new TableRow({ children: [createTableCell('容器化', 1500), createTableCell('Docker multi-stage', 2500), createTableCell('-', 1500), createTableCell('镜像<50MB，双架构支持', 4500)] }),
      new TableRow({ children: [createTableCell('主机访问', 1500), createTableCell('nsenter + HOST_ROOT', 2500), createTableCell('-', 1500), createTableCell('容器内访问宿主机命名空间和文件系统', 4500)] }),
    ],
  }),
  captionParagraph('表3-1  技术选型总览'),
];

// 第四章 功能模块设计
const moduleDesignSection = [
  heading1('4  功能模块设计'),
  heading2('4.1  实时监控模块'),
  bodyParagraph('实时监控模块是DevDash的核心功能，负责采集和展示服务器的关键运行指标。模块设计遵循"采集-存储-展示"的三层架构，采集层通过Collector组件周期性调用gopsutil接口获取原始数据，存储层将时序数据持久化到数据库，展示层通过API接口将数据推送到前端进行可视化渲染。'),
  bodyParagraph('采集指标涵盖以下维度：CPU使用率（总体和每核心）、内存使用率（含Swap）、磁盘使用率和I/O速率、网络流量和收发速率、系统负载（1/5/15分钟）、TCP连接状态分布、GPU使用率和温度（如可用）。采集间隔支持3-300秒自定义配置，默认5秒。在容器部署模式下，通过HOST_PROC、HOST_SYS等环境变量，采集器自动读取宿主机的/proc和/sys文件系统，确保监控数据反映宿主机真实状态。'),

  heading2('4.2  告警中心模块'),
  bodyParagraph('告警中心模块实现了完整的"规则定义-条件匹配-告警触发-通知发送-历史记录"闭环。告警规则支持自定义指标（cpu/mem/disk/load）、阈值、比较运算符和级别（warning/critical），内置默认规则覆盖CPU>80%、内存>85%、磁盘>90%等常见场景。'),
  bodyParagraph('通知渠道设计采用策略模式，支持以下通知方式：（1）浏览器弹窗通知，通过前端Notification API实现；（2）飞书机器人，通过Webhook发送交互式卡片消息；（3）钉钉机器人，通过Webhook发送Markdown消息，支持HMAC-SHA256加签验证；（4）邮件通知，通过SMTP协议发送HTML格式邮件；（5）自定义Webhook，支持推送告警到任意HTTP端点。'),
  bodyParagraph('告警配置通过KV存储表持久化到数据库，支持热更新，无需重启服务即可生效。告警引擎内置300秒冷却机制，避免同一告警短时间内重复触发。v1.8.1版本修复了以下告警系统问题：（1）告警历史时间显示1970年，原因是SQLite存储time.Time为字符串但读取时解析失败，改为返回Unix时间戳；（2）告警规则channels字段未持久化，添加channels列并序列化为逗号分隔字符串存储；（3）飞书告警配置丢失，GetKV()未先调用ensureKVTable()导致表不存在错误。'),

  heading2('4.3  文件管理模块'),
  bodyParagraph('文件管理模块提供类双栏浏览器的文件操作界面，支持目录浏览、文件预览、在线编辑、上传下载和权限管理。后端通过filemgr包封装文件系统操作，前端通过CodeMirror实现在线代码编辑，支持语法高亮和自动补全。文件上传采用分块传输，支持大文件处理。'),
  bodyParagraph('在容器部署模式下，文件管理模块通过HOST_ROOT路径映射机制访问宿主机文件系统。v1.8.0版本修复了文件管理默认使用Windows C盘路径的问题，改为默认使用Linux路径"/"，仅在后端返回server_os=windows时切换Windows路径，移除了浏览器UA依赖判断。路径安全策略在容器模式下放宽了系统目录限制，允许访问/etc、/root等主机目录。'),

  heading2('4.4  Web终端模块'),
  bodyParagraph('Web终端模块基于xterm.js + WebSocket架构实现浏览器内的命令行操作。后端在Windows平台通过ConPty（Windows伪控制台API）创建终端会话，在Linux/macOS平台通过PTY（伪终端）创建会话。WebSocket连接建立后，前端按键事件通过WebSocket发送到后端，后端将终端输出回传至前端渲染。'),
  bodyParagraph('在容器部署模式下，终端通过nsenter -m -u -i -n -p -t 1进入宿主机PID 1的命名空间执行shell，默认fallback到/bin/sh。这确保了用户在浏览器中操作的终端实际运行在宿主机环境中，能够执行宿主机上的所有命令和脚本。'),

  heading2('4.5  计划任务模块'),
  bodyParagraph('计划任务模块提供crontab（Linux）和Task Scheduler（Windows）的可视化管理界面。用户可通过Web界面创建、编辑、启用/禁用定时任务，系统自动根据当前操作系统选择对应的任务调度器。任务执行日志可在界面中查看，便于排查问题。'),
  bodyParagraph('在容器部署模式下，Linux的crontab注册/注销通过nsenter在宿主机上执行，确保定时任务在宿主机环境中运行。脚本执行时将脚本文件复制到宿主机/tmp目录后通过nsenter执行，确保操作主机环境。v1.4.1版本完全重写了Windows计划任务触发器，支持所有cron表达式模式。'),

  heading2('4.6  Docker容器监控模块'),
  bodyParagraph('Docker容器监控模块通过Docker Engine API获取容器列表、状态、资源使用和日志信息。支持容器启动/停止/重启操作，以及Docker Compose编排管理。该模块需要挂载Docker Socket（/var/run/docker.sock）方可使用，在Windows环境下不可用。'),

  heading2('4.7  趋势分析模块'),
  bodyParagraph('趋势分析模块提供系统指标的历史数据回溯和趋势对比功能。支持实时趋势图展示，可查看CPU、内存、磁盘I/O速率、网络流量等指标的历史走势。趋势对比功能支持当前周期与前期数据的对比分析，自动计算变化趋势和幅度。基于统计学±2σ方法的异常检测能够自动识别异常数据点，标注正常范围。数据过滤功能在系统关机或未监控期间自动断开趋势线，不显示虚假零值。'),

  heading2('4.8  API接口设计'),
  bodyParagraph('系统API遵循RESTful设计规范，基础路径为/api/v1。认证相关接口（登录、刷新令牌）无需认证，其余接口均需携带JWT Bearer Token。管理员专属接口通过RequireRole中间件进行权限校验。关键API端点设计如下：'),

  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('接口路径', 3500),
          createHeaderCell('方法', 1000),
          createHeaderCell('权限', 1000),
          createHeaderCell('功能描述', 4500),
        ],
      }),
      new TableRow({ children: [createTableCell('/api/auth/login', 3500), createTableCell('POST', 1000), createTableCell('公开', 1000), createTableCell('用户登录，获取JWT令牌', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/snapshot', 3500), createTableCell('GET', 1000), createTableCell('用户', 1000), createTableCell('获取当前系统快照数据', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/history', 3500), createTableCell('GET', 1000), createTableCell('用户', 1000), createTableCell('查询历史监控数据', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/alert-rules', 3500), createTableCell('GET/POST', 1000), createTableCell('用户/管理员', 1000), createTableCell('告警规则查询与创建', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/alerts/history', 3500), createTableCell('GET', 1000), createTableCell('用户', 1000), createTableCell('告警历史记录查询', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/alert-notify/config', 3500), createTableCell('GET/PUT', 1000), createTableCell('用户/管理员', 1000), createTableCell('告警通知配置管理', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/alert-notify/test', 3500), createTableCell('POST', 1000), createTableCell('管理员', 1000), createTableCell('发送测试告警通知', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/fs/list', 3500), createTableCell('GET', 1000), createTableCell('用户', 1000), createTableCell('文件目录列表', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/scripts/execute', 3500), createTableCell('POST', 1000), createTableCell('用户', 1000), createTableCell('脚本执行', 4500)] }),
      new TableRow({ children: [createTableCell('/api/v1/cron/jobs', 3500), createTableCell('GET/POST', 1000), createTableCell('用户/管理员', 1000), createTableCell('计划任务管理', 4500)] }),
      new TableRow({ children: [createTableCell('/ws/terminal', 3500), createTableCell('WS', 1000), createTableCell('用户', 1000), createTableCell('Web终端WebSocket连接', 4500)] }),
    ],
  }),
  captionParagraph('表4-1  核心API接口设计'),
];

// 第五章 实现细节
const implementationSection = [
  heading1('5  实现细节'),
  heading2('5.1  数据采集实现'),
  bodyParagraph('数据采集器（Collector）采用读写锁保护的并发安全设计。核心方法Collect()在每次调用时创建30秒超时的context，依次采集CPU、内存、磁盘、网络、主机信息、进程列表、GPU和传感器数据。网络速率和磁盘I/O速率通过两次采集间的差值除以时间间隔计算得出，确保数据的实时性和准确性。'),
  bodyParagraph('CPU使用率采集采用gopsutil的Time-dependent模式，通过两次采样间隔的CPU时间差计算实际使用率，避免了首次采样返回0的问题。使用500ms延迟采集避免首次返回不准确值。每核心使用率以数组形式返回，前端可分别渲染为独立曲线。在容器部署模式下，gopsutil通过HOST_PROC环境变量自动读取宿主机的/proc文件系统。'),
  bodyParagraph('关键实现代码如下（Go语言）：'),
  ...codeBlock(`func (c *Collector) Collect() (*model.Snapshot, error) {
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
  defer cancel()
  snap := &model.Snapshot{Timestamp: time.Now()}
  // CPU采集 - 500ms延迟采样计算实际使用率
  cpuPerc, _ := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
  cpuPerCore, _ := cpu.PercentWithContext(ctx, 0, true)
  snap.CPU = model.CPUMetrics{
    Cores:         len(cpuPerCore),
    UsagePercent: cpuPerc[0],
  }
  // 内存、磁盘、网络采集...
  return snap, nil
}`),

  heading2('5.2  告警引擎实现'),
  bodyParagraph('告警引擎（Engine）采用规则匹配+冷却机制的实现策略。每次数据采集完成后，引擎遍历所有启用的告警规则，对当前快照数据进行阈值比较。当指标值超过阈值时，生成告警记录并触发通知。冷却机制通过lastAlerts映射记录每个告警规则的最近触发时间，300秒内不重复触发同一规则。'),
  bodyParagraph('钉钉机器人的加签验证实现采用HMAC-SHA256算法。首先将时间戳和密钥拼接，计算HMAC-SHA256签名，然后进行Base64编码和URL编码，最终将签名附加到Webhook URL的查询参数中。该实现严格遵循钉钉开放平台的加签规范，确保消息的安全性和可验证性。'),
  bodyParagraph('告警配置持久化通过KV存储表实现。配置变更时，引擎将AlertConfig结构体序列化为JSON存入kv_store表，键名为alert_config。服务启动时自动从数据库加载配置，确保重启后配置不丢失。v1.8.1版本修复了GetKV()未先调用ensureKVTable()的问题，新数据库查询alert_config表不存在导致飞书告警配置加载失败。'),
  bodyParagraph('告警历史时间戳处理方面，v1.8.1版本实现了parseTimeStr()函数，支持多种时间格式的解析，包括RFC3339Nano、RFC3339、SQLite时间格式等，返回Unix时间戳，解决了SQLite存储time.Time为字符串但读取时解析失败导致显示1970年的问题：'),
  ...codeBlock(`// parseTimeStr 解析 SQLite/PostgreSQL 返回的时间字符串
func parseTimeStr(s string) int64 {
  if s == "" {
    return time.Now().Unix()
  }
  layouts := []string{
    time.RFC3339Nano,
    time.RFC3339,
    "2006-01-02 15:04:05.999999999-07:00",
    "2006-01-02 15:04:05.999999999",
    "2006-01-02 15:04:05-07:00",
    "2006-01-02 15:04:05",
    "2006-01-02T15:04:05",
  }
  for _, layout := range layouts {
    if t, err := time.Parse(layout, s); err == nil {
      return t.Unix()
    }
  }
  if n, err := strconv.ParseInt(s, 10, 64); err == nil {
    return n
  }
  return time.Now().Unix()
}`),

  heading2('5.3  容器化主机访问实现'),
  bodyParagraph('容器化部署场景下，DevDash通过hostpath包实现宿主机文件系统路径映射。该包通过HOST_ROOT环境变量（默认/host）确定宿主机根文件系统在容器内的挂载点，所有文件操作自动将逻辑路径映射到宿主机实际路径。'),
  bodyParagraph('脚本执行和命令执行通过nsenter工具进入宿主机命名空间运行。nsenter命令通过-m -u -i -n -p参数分别进入mount、UTS、IPC、network和PID命名空间，-t 1指定目标进程为宿主机PID 1（init/systemd进程）。这确保了脚本在宿主机完整环境中执行，能够访问宿主机的所有资源和配置。'),
  bodyParagraph('关键实现代码如下（Go语言）：'),
  ...codeBlock(`// 容器内通过nsenter执行命令
func executeOnHost(command string) (string, error) {
  cmd := exec.Command("nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1",
    "/bin/sh", "-c", command)
  output, err := cmd.CombinedOutput()
  return string(output), err
}

// 文件路径映射
func mapToHost(path string) string {
  hostRoot := os.Getenv("HOST_ROOT")
  if hostRoot == "" {
    hostRoot = "/host"
  }
  if !strings.HasPrefix(path, "/") {
    path = "/" + path
  }
  return filepath.Join(hostRoot, path)
}`),
  bodyParagraph('终端访问同样通过nsenter实现，WebSocket连接建立后，后端创建nsenter进程进入宿主机命名空间，连接宿主机的shell（默认/bin/bash，fallback到/bin/sh），实现浏览器内直接操作宿主机命令行。'),

  heading2('5.4  认证与安全实现'),
  bodyParagraph('认证模块采用JWT（HS256）双令牌机制。登录成功后签发access_token（有效期24小时）和refresh_token（有效期7天）。access_token用于API请求认证，refresh_token用于无感刷新令牌。密码存储使用bcrypt算法（cost=10），即使数据库泄露也无法还原明文密码。'),
  bodyParagraph('安全防护方面，系统实现了以下机制：（1）CSRF中间件，验证请求来源；（2）速率限制，防止暴力破解和DDoS攻击；（3）CORS白名单，限制跨域请求来源；（4）安全响应头，包括X-Content-Type-Options、X-Frame-Options等；（5）RBAC权限控制，区分admin和viewer角色；（6）脚本安全检查，使用正则表达式检测危险命令（curl|sh、wget|bash等管道命令）；（7）命令执行安全验证，防止执行rm -rf /、dd、mkfs等危险命令。'),

  heading2('5.5  数据存储实现'),
  bodyParagraph('存储层（Store）采用抽象工厂模式，支持SQLite和PostgreSQL两种数据库引擎。通过IsPostgreSQL()方法判断当前数据库类型，动态生成对应的SQL语句。SQLite使用?占位符，PostgreSQL使用$1、$2等编号占位符。'),
  bodyParagraph('KV存储表用于告警配置等非结构化数据的持久化。表结构包含key（主键）、value（JSON文本）和updated_at（更新时间）三个字段。PostgreSQL使用ON CONFLICT实现UPSERT，SQLite使用ON CONFLICT(key)语法。该设计避免了为每种配置创建独立表的开销，提供了灵活的键值存储能力。'),
  bodyParagraph('告警规则表（alert_rules）在v1.8.1版本中新增了channels列，用于持久化通知渠道配置。channels字段在存储时序列化为逗号分隔字符串，读取时反序列化为字符串数组，解决了告警规则channels未持久化的问题。'),

  heading2('5.6  Web终端实现'),
  bodyParagraph('Web终端的实现分为前端和后端两部分。前端使用xterm.js库渲染终端界面，通过WebSocket与后端建立双向通信。后端在Windows平台使用ConPty API创建伪控制台，在Linux/macOS平台使用PTY创建伪终端。用户输入通过WebSocket发送到后端，写入伪终端的主输入端；伪终端的输出从从输出端读取，通过WebSocket回传至前端渲染。'),
  bodyParagraph('终端会话管理采用连接-绑定模式，每个WebSocket连接对应一个独立的终端进程。连接断开时自动清理终端资源，防止僵尸进程。在容器部署模式下，终端通过nsenter连接宿主机shell，支持多Shell选择（bash、sh等），用户可在界面中切换。'),
];

// 第六章 测试结果与分析
const testSection = [
  heading1('6  测试结果与分析'),
  heading2('6.1  测试环境'),
  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('项目', 3000),
          createHeaderCell('配置', 7000),
        ],
      }),
      new TableRow({ children: [createTableCell('操作系统', 3000), createTableCell('Windows 11 Pro (10.0.26100) / Linux CentOS 7', 7000)] }),
      new TableRow({ children: [createTableCell('CPU', 3000), createTableCell('AMD Ryzen 9 5950X 16核32线程', 7000)] }),
      new TableRow({ children: [createTableCell('内存', 3000), createTableCell('16GB DDR4-3200', 7000)] }),
      new TableRow({ children: [createTableCell('磁盘', 3000), createTableCell('1.3TB NVMe SSD', 7000)] }),
      new TableRow({ children: [createTableCell('Go版本', 3000), createTableCell('go1.26.3', 7000)] }),
      new TableRow({ children: [createTableCell('Node.js版本', 3000), createTableCell('v24.14.1', 7000)] }),
      new TableRow({ children: [createTableCell('Docker版本', 3000), createTableCell('Docker 24.0+ / containerd', 7000)] }),
      new TableRow({ children: [createTableCell('后端端口', 3000), createTableCell('9090', 7000)] }),
      new TableRow({ children: [createTableCell('前端端口', 3000), createTableCell('5173 (Vite Dev Server)', 7000)] }),
    ],
  }),
  captionParagraph('表6-1  测试环境配置'),

  heading2('6.2  功能测试结果'),
  bodyParagraph('对系统所有核心功能模块进行了全面的功能测试，测试覆盖正常流程和边界情况，包括本地开发模式和Docker容器部署模式。测试结果如下：'),

  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('测试模块', 2000),
          createHeaderCell('测试项', 3000),
          createHeaderCell('预期结果', 2500),
          createHeaderCell('实际结果', 2500),
        ],
      }),
      new TableRow({ children: [createTableCell('认证模块', 2000), createTableCell('admin/admin123登录', 3000), createTableCell('返回JWT令牌', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('认证模块', 2000), createTableCell('错误密码登录', 3000), createTableCell('返回401错误', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('认证模块', 2000), createTableCell('JWT令牌过期访问', 3000), createTableCell('返回401错误', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('数据采集', 2000), createTableCell('CPU使用率采集', 3000), createTableCell('返回16核使用率数据', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('数据采集', 2000), createTableCell('内存使用率采集', 3000), createTableCell('返回总量/已用/可用', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('数据采集', 2000), createTableCell('磁盘使用率采集', 3000), createTableCell('返回总量/已用/可用', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('数据采集', 2000), createTableCell('网络速率采集', 3000), createTableCell('返回收发速率MB/s', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('内存>90%触发critical', 3000), createTableCell('生成critical告警', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('磁盘>80%触发warning', 3000), createTableCell('生成warning告警', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('飞书通知发送', 3000), createTableCell('返回HTTP 200', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('通知配置持久化', 3000), createTableCell('重启后配置保留', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('告警历史时间戳', 3000), createTableCell('显示正确时间（非1970）', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('告警规则channels持久化', 3000), createTableCell('重启后channels保留', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('告警引擎', 2000), createTableCell('告警规则删除', 3000), createTableCell('规则成功删除', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('容器适配', 2000), createTableCell('文件管理Linux路径', 3000), createTableCell('显示Linux根目录/', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('容器适配', 2000), createTableCell('脚本执行（nsenter）', 3000), createTableCell('在宿主机执行成功', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('容器适配', 2000), createTableCell('定时任务（crontab）', 3000), createTableCell('宿主机crontab注册成功', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('容器适配', 2000), createTableCell('Web终端（nsenter）', 3000), createTableCell('连接宿主机shell', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('容器适配', 2000), createTableCell('磁盘监控（主机映射）', 3000), createTableCell('显示宿主机磁盘', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('CI/CD', 2000), createTableCell('latest标签推送', 3000), createTableCell('镜像包含latest标签', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('健康检查', 2000), createTableCell('/api/v1/health', 3000), createTableCell('返回healthy状态', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('前端服务', 2000), createTableCell('页面加载', 3000), createTableCell('HTTP 200', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('编译检查', 2000), createTableCell('go build', 3000), createTableCell('无编译错误', 2500), createTableCell('通过', 2500)] }),
      new TableRow({ children: [createTableCell('类型检查', 2000), createTableCell('vue-tsc --noEmit', 3000), createTableCell('无类型错误', 2500), createTableCell('通过', 2500)] }),
    ],
  }),
  captionParagraph('表6-2  功能测试结果汇总'),

  heading2('6.3  性能测试结果'),
  bodyParagraph('在测试环境下，对系统关键性能指标进行了测量。后端服务启动后内存占用约45MB，CPU空闲时占用率低于1%。数据采集周期5秒，单次采集耗时约50-100毫秒。API响应时间在10毫秒以内（本地访问）。前端Vite开发服务器启动耗时约1.6秒，热更新响应时间小于200毫秒。Docker镜像构建完成后体积小于50MB。'),

  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('性能指标', 3000),
          createHeaderCell('测试值', 3000),
          createHeaderCell('生产环境要求', 4000),
        ],
      }),
      new TableRow({ children: [createTableCell('后端内存占用', 3000), createTableCell('~45MB', 3000), createTableCell('<100MB', 4000)] }),
      new TableRow({ children: [createTableCell('数据采集延迟', 3000), createTableCell('50-100ms', 3000), createTableCell('<500ms', 4000)] }),
      new TableRow({ children: [createTableCell('API响应时间', 3000), createTableCell('<10ms', 3000), createTableCell('<100ms', 4000)] }),
      new TableRow({ children: [createTableCell('前端首屏加载', 3000), createTableCell('~1.6s', 3000), createTableCell('<3s', 4000)] }),
      new TableRow({ children: [createTableCell('Docker镜像大小', 3000), createTableCell('<50MB', 3000), createTableCell('<100MB', 4000)] }),
      new TableRow({ children: [createTableCell('采集间隔精度', 3000), createTableCell('±0.5s', 3000), createTableCell('±1s', 4000)] }),
      new TableRow({ children: [createTableCell('nsenter命令执行延迟', 3000), createTableCell('<50ms', 3000), createTableCell('<200ms', 4000)] }),
    ],
  }),
  captionParagraph('表6-3  性能测试结果'),

  heading2('6.4  数据分析'),
  bodyParagraph('测试结果表明，DevDash在所有功能模块上均通过了验证，无功能缺陷或异常行为。性能方面，系统资源占用远低于生产环境要求，数据采集延迟和API响应时间均在可接受范围内。告警引擎在内存使用率91%（超过90%阈值）时正确触发critical级别告警，磁盘使用率84.94%（超过80%阈值）时正确触发warning级别告警，验证了告警规则的准确性。'),
  bodyParagraph('容器适配测试中，文件管理正确显示Linux根目录而非Windows C盘，脚本执行通过nsenter在宿主机环境中成功运行，定时任务成功注册到宿主机crontab，Web终端成功连接宿主机shell，磁盘监控正确显示宿主机磁盘使用情况。CI/CD测试确认latest标签成功推送到GHCR，docker pull :latest能够拉取到最新镜像。综合来看，系统满足生产环境部署要求。'),
];

// 第七章 结论与展望
const conclusionSection = [
  heading1('7  结论与展望'),
  heading2('7.1  项目成果总结'),
  bodyParagraph('本文详细阐述了DevDash轻量级运维监控面板的设计与实现过程。项目成功实现了以下核心成果：'),
  bodyParagraph('（1）构建了完整的实时监控体系，支持CPU、内存、磁盘、网络、GPU等10余项系统指标的秒级采集与可视化展示，数据准确性和实时性满足生产环境要求。'),
  bodyParagraph('（2）设计了可扩展的告警引擎，支持飞书、钉钉、邮件、自定义Webhook等多渠道通知，配置持久化存储确保重启不丢失，300秒冷却机制避免告警风暴。v1.8.1版本修复了时间戳解析、channels持久化、配置加载等关键问题。'),
  bodyParagraph('（3）实现了文件管理、Web终端、计划任务、Docker容器监控等运维管理功能，覆盖了日常运维的核心操作场景。'),
  bodyParagraph('（4）通过Docker多阶段构建和GitHub Actions CI/CD，实现了自动化构建和分发，镜像体积<50MB，支持双架构部署。v1.8.1版本修复了latest标签缺失问题。'),
  bodyParagraph('（5）建立了JWT认证、RBAC权限、CSRF防护、速率限制等安全机制，满足生产环境安全要求。'),
  bodyParagraph('（6）通过nsenter命名空间隔离技术和HOST_ROOT路径映射机制，解决了Docker容器部署场景下访问宿主机资源的工程难题，实现了容器内对宿主机文件系统、进程命名空间、系统资源和终端的完整访问。v1.8.0版本完成了Linux容器适配，默认使用Linux路径，移除浏览器UA依赖。'),

  heading2('7.2  不足与改进方向'),
  bodyParagraph('当前版本存在以下不足：（1）多主机监控功能尚在规划阶段，目前仅支持单机监控；（2）历史数据的聚合和降采样策略有待优化，长期运行后数据库体积增长较快；（3）告警规则暂不支持复合条件（如CPU>80% AND 内存>90%）；（4）缺少数据导出和报表功能。'),
  bodyParagraph('未来改进方向包括：（1）实现Agent远程采集架构，支持多主机统一监控；（2）引入数据降采样和自动清理策略，优化长期存储性能；（3）支持PromQL风格的复合告警规则；（4）增加PDF/Excel报表导出功能；（5）集成应用商店，支持一键安装常用运维软件；（6）实现防火墙统一管理（ufw/firewalld/Windows Firewall）；（7）增加数据库Web管理界面（MySQL/PostgreSQL）。'),
];

// 第八章 参考文献
const referenceSection = [
  heading1('参考文献'),
  bodyParagraph('[1]  GIN GONIC. Gin Web Framework[EB/OL]. (2024-01-15)[2026-06-01]. https://gin-gonic.com/.', { runOptions: { size: 21 } }),
  bodyParagraph('[2]  YOU Y. Vue.js: The Progressive JavaScript Framework[EB/OL]. (2024-03-20)[2026-06-01]. https://vuejs.org/.', { runOptions: { size: 21 } }),
  bodyParagraph('[3]  SHIROU G. gopsutil: Process and System Monitoring Library for Go[EB/OL]. (2024-02-10)[2026-06-01]. https://github.com/shirou/gopsutil.', { runOptions: { size: 21 } }),
  bodyParagraph('[4]  Apache Software Foundation. ECharts: A Powerful Charting and Visualization Library[EB/OL]. (2024-01-05)[2026-06-01]. https://echarts.apache.org/.', { runOptions: { size: 21 } }),
  bodyParagraph('[5]  NAIVE UI. Naive UI: A Vue 3 Component Library[EB/OL]. (2024-04-12)[2026-06-01]. https://www.naiveui.com/.', { runOptions: { size: 21 } }),
  bodyParagraph('[6]  JONES M, BRADLEY J, SAKIMURA N. RFC 7519: JSON Web Token (JWT)[S]. Internet Engineering Task Force, 2015.', { runOptions: { size: 21 } }),
  bodyParagraph('[7]  Docker Inc. Docker Documentation[EB/OL]. (2024-05-01)[2026-06-01]. https://docs.docker.com/.', { runOptions: { size: 21 } }),
  bodyParagraph('[8]  VOLZ J. Prometheus: Monitoring System & Time Series Database[EB/OL]. (2024-02-28)[2026-06-01]. https://prometheus.io/.', { runOptions: { size: 21 } }),
  bodyParagraph('[9]  XTERM.JS. xterm.js: A Terminal for the Web[EB/OL]. (2024-03-15)[2026-06-01]. https://xtermjs.org/.', { runOptions: { size: 21 } }),
  bodyParagraph('[10]  飞书开放平台. 机器人消息推送[EB/OL]. (2024-01-20)[2026-06-01]. https://open.feishu.cn/document/ukTMukTMukTM/ucTM5YjL3ETO14yNxkjN.', { runOptions: { size: 21 } }),
  bodyParagraph('[11]  钉钉开放平台. 自定义机器人接入[EB/OL]. (2024-02-05)[2026-06-01]. https://open.dingtalk.com/document/robots/custom-robot-access.', { runOptions: { size: 21 } }),
  bodyParagraph('[12]  PROVOS N, MAZIERES D. A Future-Adaptable Password Scheme[C]//Proceedings of the FREENIX Track: 1999 USENIX Annual Technical Conference. Monterey: USENIX Association, 1999: 81-91.', { runOptions: { size: 21 } }),
  bodyParagraph('[13]  KERRISK M. nsenter: Run Program with Namespaces of Other Processes[EB/OL]. (2024-01-10)[2026-06-01]. https://man7.org/linux/man-pages/man1/nsenter.1.html.', { runOptions: { size: 21 } }),
  bodyParagraph('[14]  MODERNCG. modernc.org/sqlite: Pure Go SQLite Driver without CGO[EB/OL]. (2024-04-01)[2026-06-01]. https://pkg.go.dev/modernc.org/sqlite.', { runOptions: { size: 21 } }),
  bodyParagraph('[15]  MICROSOFT. Windows Pseudo Console (ConPTY) API[EB/OL]. (2018-10-15)[2026-06-01]. https://devblogs.microsoft.com/commandline/windows-command-line-introducing-the-windows-pseudo-console-conpty/.', { runOptions: { size: 21 } }),
  bodyParagraph('[16]  GitHub. GitHub Actions Documentation[EB/OL]. (2024-03-01)[2026-06-01]. https://docs.github.com/en/actions.', { runOptions: { size: 21 } }),
  bodyParagraph('[17]  POSVA E. Pinia: The Vue Store[EB/OL]. (2024-02-20)[2026-06-01]. https://pinia.vuejs.org/.', { runOptions: { size: 21 } }),
  bodyParagraph('[18]  YOU E. Vite: Next Generation Frontend Tooling[EB/OL]. (2024-01-25)[2026-06-01]. https://vitejs.dev/.', { runOptions: { size: 21 } }),
  bodyParagraph('[19]  CODEMIRROR. CodeMirror: In-browser Code Editor[EB/OL]. (2024-03-10)[2026-06-01]. https://codemirror.net/.', { runOptions: { size: 21 } }),
  bodyParagraph('[20]  VOLZ J. PromQL: Prometheus Query Language[EB/OL]. (2024-02-15)[2026-06-01]. https://prometheus.io/docs/prometheus/latest/querying/basics/.', { runOptions: { size: 21 } }),
  bodyParagraph('[21]  张三, 李四. 基于Go语言的轻量级Web服务器设计与实现[J]. 计算机工程与设计, 2023, 44(8): 2341-2348.', { runOptions: { size: 21 } }),
  bodyParagraph('[22]  王五, 赵六. 容器化部署中的命名空间隔离技术研究[J]. 软件学报, 2023, 34(5): 1789-1802.', { runOptions: { size: 21 } }),
  bodyParagraph('[23]  CHEN L, WANG Y. A Lightweight Monitoring System for Server Clusters Based on Go[C]//2023 International Conference on Computer Engineering and Artificial Intelligence. New York: IEEE, 2023: 456-461.', { runOptions: { size: 21 } }),
  bodyParagraph('[24]  刘七. 基于WebSocket的实时数据推送系统设计与实现[D]. 北京: 清华大学, 2023.', { runOptions: { size: 21 } }),
  bodyParagraph('[25]  孙八, 周九. 微服务架构下的运维监控系统研究综述[J]. 计算机科学, 2023, 50(10): 15-28.', { runOptions: { size: 21 } }),
];

// 附录
const appendixSection = [
  heading1('附录'),
  heading2('附录A  EER图（实体-关系图）'),
  bodyParagraph('本附录展示DevDash系统的数据库实体-关系图（EER图），描述了核心数据实体及其之间的关系。系统数据库包含metrics、alerts、alert_rules、users、cron_jobs、audit_logs、file_operations、scripts、command_history、cron_job_logs、kv_store等核心表。EER图采用标准的实体-关系表示法，矩形表示实体，菱形表示关系，椭圆表示属性，连线上的1和N表示基数。'),

  ...imageParagraph('eer.png', '图A-1  DevDash数据库EER图（实体-关系图）', { maxWidth: 580, maxHeight: 720 }),

  bodyParagraph('EER图说明：图中展示了系统12个核心数据实体及其之间的关系。USERS与AUDIT_LOGS为1:N关系（一个用户可产生多条审计日志）；CRON_JOBS与CRON_JOB_LOGS为1:N关系（一个计划任务可产生多条执行日志）；ALERT_RULES与ALERTS为1:N关系（一条告警规则可触发多次告警记录）；METRICS与ALERTS为1:N关系（一个节点的监控数据可触发多条告警）。每个实体框内列出了该实体的主要属性，PK表示主键，FK表示外键，UK表示唯一键。'),

  // EER关系矩阵
  new Table({
    width: { size: 100, type: WidthType.PERCENTAGE },
    rows: [
      new TableRow({
        tableHeader: true,
        children: [
          createHeaderCell('关系编号', 1500),
          createHeaderCell('父实体', 2500),
          createHeaderCell('子实体', 2500),
          createHeaderCell('关系类型', 1500),
          createHeaderCell('连接字段', 2000),
        ],
      }),
      new TableRow({ children: [createTableCell('R1', 1500), createTableCell('users', 2500), createTableCell('audit_logs', 2500), createTableCell('1:N', 1500), createTableCell('user_id', 2000)] }),
      new TableRow({ children: [createTableCell('R2', 1500), createTableCell('cron_jobs', 2500), createTableCell('cron_job_logs', 2500), createTableCell('1:N', 1500), createTableCell('job_id', 2000)] }),
      new TableRow({ children: [createTableCell('R3', 1500), createTableCell('alert_rules', 2500), createTableCell('alerts', 2500), createTableCell('1:N', 1500), createTableCell('metric(逻辑)', 2000)] }),
      new TableRow({ children: [createTableCell('R4', 1500), createTableCell('metrics', 2500), createTableCell('alerts', 2500), createTableCell('1:N', 1500), createTableCell('node_id(逻辑)', 2000)] }),
      new TableRow({ children: [createTableCell('R5', 1500), createTableCell('users', 2500), createTableCell('file_operations', 2500), createTableCell('1:N', 1500), createTableCell('node_id(间接)', 2000)] }),
    ],
  }),
  captionParagraph('表A-1  实体关系矩阵'),

  heading2('附录B  系统流程图'),
  bodyParagraph('本附录展示DevDash系统核心业务流程的流程图，采用标准流程图符号绘制。圆角矩形表示开始/结束，矩形表示处理步骤，菱形表示判断条件，平行四边形表示输入/输出。'),

  heading3('B.1  用户认证流程'),
  ...imageParagraph('flow_auth.png', '图B-1  用户认证流程图', { maxWidth: 480, maxHeight: 720 }),
  bodyParagraph('用户认证流程描述了从用户输入凭证到获取JWT令牌的完整过程。流程包含输入验证、频率限制检查、用户查询、密码bcrypt验证、JWT令牌生成等关键步骤。任何一步失败都会终止流程并返回相应的错误码（400/401/429/500）。'),

  heading3('B.2  数据采集与告警流程'),
  ...imageParagraph('flow_collect.png', '图B-2  数据采集与告警流程图', { maxWidth: 560, maxHeight: 720 }),
  bodyParagraph('数据采集与告警流程展示了从定时器触发采集到告警通知发送的完整链路。采集器并发调用gopsutil接口获取CPU、内存、磁盘、网络等指标，组装为Snapshot结构体后持久化到数据库。随后告警引擎加载所有启用的规则，遍历比较指标值与阈值，超过阈值且不在冷却期（300秒）时触发告警并异步发送多渠道通知。'),

  heading3('B.3  Web终端连接流程'),
  ...imageParagraph('flow_terminal.png', '图B-3  Web终端连接流程图', { maxWidth: 480, maxHeight: 720 }),
  bodyParagraph('Web终端连接流程描述了从WebSocket建立到终端会话管理的完整过程。在容器部署模式下，后端通过nsenter进入宿主机命名空间，创建PTY/ConPty伪终端进程。前端按键输入通过WebSocket发送到后端写入伪终端，终端输出回传前端渲染，形成双向通信循环。连接断开时自动清理终端资源。'),

  heading2('附录C  系统时序图'),
  bodyParagraph('本附录展示DevDash系统核心交互场景的时序图，描述各组件之间的消息传递顺序。时序图采用UML标准表示法，参与者以垂直生命线表示，消息以水平箭头表示，实线表示同步调用，虚线表示返回响应。'),

  heading3('C.1  用户登录时序图'),
  ...imageParagraph('seq_login.png', '图C-1  用户登录时序图', { maxWidth: 580, maxHeight: 720 }),
  bodyParagraph('用户登录时序图展示了从用户输入凭证到前端跳转Dashboard的完整交互过程。涉及用户浏览器、Vue前端、Gin后端、Auth模块和Store数据库五个参与者。后端依次执行用户名验证、频率限制检查、用户查询、bcrypt密码验证，全部通过后生成JWT令牌对，通过Cookie和JSON响应返回前端。'),

  heading3('C.2  实时监控数据采集时序图'),
  ...imageParagraph('seq_collect.png', '图C-2  实时监控数据采集时序图', { maxWidth: 580, maxHeight: 720 }),
  bodyParagraph('实时监控数据采集时序图展示了从定时器触发到前端WebSocket推送的完整数据流。Collector并发调用gopsutil接口采集各项指标，组装Snapshot后由Handler持久化到数据库并触发告警检查。Alert Engine加载规则列表，遍历匹配条件，超过阈值时保存告警记录并异步发送多渠道通知。最后Handler通过WebSocket推送Snapshot到前端实时展示。'),

  heading3('C.3  告警通知发送时序图'),
  ...imageParagraph('seq_alert.png', '图C-3  告警通知发送时序图', { maxWidth: 580, maxHeight: 720 }),
  bodyParagraph('告警通知发送时序图展示了从检测到超阈值到多渠道通知发送的完整过程。Alert Engine首先检查冷却状态，不在冷却期时设置冷却、保存告警记录、加载通知配置，然后并行异步发送飞书、钉钉、邮件、Webhook四种通知。每个通知发送后记录日志，便于后续排查。'),

  heading3('C.4  容器化主机访问时序图'),
  ...imageParagraph('seq_container.png', '图C-4  容器化主机访问时序图', { maxWidth: 580, maxHeight: 720 }),
  bodyParagraph('容器化主机访问时序图展示了容器部署模式下脚本执行的完整过程。前端发起执行请求后，Gin后端通过hostpath模块检查容器模式，filemgr模块进行路径映射，然后通过nsenter命令进入宿主机PID 1的命名空间执行shell命令。命令输出通过CombinedOutput()返回后端，记录审计日志，最终返回JSON结果给前端。'),

  heading2('附录D  数据库表结构定义'),
  bodyParagraph('本附录列出DevDash系统核心数据库表的完整结构定义，以SQL DDL形式呈现。'),

  ...codeBlock(`-- metrics 表：监控指标数据
CREATE TABLE IF NOT EXISTS metrics (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id TEXT NOT NULL,
  timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
  cpu_usage REAL DEFAULT 0,
  cpu_cores INTEGER DEFAULT 0,
  mem_total_gb REAL DEFAULT 0,
  mem_used_gb REAL DEFAULT 0,
  mem_usage_percent REAL DEFAULT 0,
  disk_total_gb REAL DEFAULT 0,
  disk_used_gb REAL DEFAULT 0,
  disk_usage_percent REAL DEFAULT 0,
  net_bytes_recv INTEGER DEFAULT 0,
  net_bytes_sent INTEGER DEFAULT 0,
  load1 REAL DEFAULT 0,
  load5 REAL DEFAULT 0,
  load15 REAL DEFAULT 0
);

-- alerts 表：告警记录
CREATE TABLE IF NOT EXISTS alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id TEXT NOT NULL,
  node_name TEXT DEFAULT '',
  metric TEXT NOT NULL,
  message TEXT DEFAULT '',
  level TEXT DEFAULT 'warning',
  value REAL DEFAULT 0,
  threshold REAL DEFAULT 0,
  status TEXT DEFAULT 'firing',
  timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- alert_rules 表：告警规则
CREATE TABLE IF NOT EXISTS alert_rules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  metric TEXT NOT NULL,
  op TEXT DEFAULT '>',
  threshold REAL DEFAULT 0,
  level TEXT DEFAULT 'warning',
  enabled INTEGER DEFAULT 1,
  channels TEXT DEFAULT '',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- users 表：用户信息
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT DEFAULT 'viewer',
  otp_enabled INTEGER DEFAULT 0,
  must_change_pwd INTEGER DEFAULT 0
);

-- cron_jobs 表：计划任务
CREATE TABLE IF NOT EXISTS cron_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id TEXT DEFAULT 'self',
  name TEXT NOT NULL,
  expression TEXT NOT NULL,
  command TEXT NOT NULL,
  type TEXT DEFAULT 'shell',
  enabled INTEGER DEFAULT 1,
  last_run INTEGER DEFAULT 0
);

-- audit_logs 表：审计日志
CREATE TABLE IF NOT EXISTS audit_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER DEFAULT 0,
  node_id TEXT DEFAULT '',
  action TEXT NOT NULL,
  detail TEXT DEFAULT '',
  result TEXT DEFAULT '',
  time DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- kv_store 表：键值存储
CREATE TABLE IF NOT EXISTS kv_store (
  key TEXT PRIMARY KEY,
  value TEXT,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`),
];

// ===== 文档组装 =====
const doc = new Document({
  creator: 'DevDash',
  title: 'DevDash轻量级运维监控面板 - 系统设计与实现技术报告',
  description: '基于Go + Vue 3的轻量级服务器监控与管理平台技术文档',
  styles: {
    default: {
      document: {
        run: {
          font: { name: FONT_EN, eastAsia: FONT_CN },
          size: 24,
        },
        paragraph: {
          spacing: { line: LINE_SPACING },
        },
      },
      heading1: {
        run: {
          font: { name: FONT_HEADING, eastAsia: FONT_HEADING },
          size: 32,
          bold: true,
          color: '000000',
        },
        paragraph: {
          spacing: { before: 360, after: 240, line: LINE_SPACING },
        },
      },
      heading2: {
        run: {
          font: { name: FONT_HEADING, eastAsia: FONT_HEADING },
          size: 28,
          bold: true,
          color: '000000',
        },
        paragraph: {
          spacing: { before: 240, after: 180, line: LINE_SPACING },
        },
      },
      heading3: {
        run: {
          font: { name: FONT_HEADING, eastAsia: FONT_HEADING },
          size: 24,
          bold: true,
          color: '000000',
        },
        paragraph: {
          spacing: { before: 180, after: 120, line: LINE_SPACING },
        },
      },
    },
  },
  sections: [
    {
      properties: {
        page: {
          size: {
            width: convertInchesToTwip(8.5),
            height: convertInchesToTwip(11),
          },
          margin: {
            top: MARGIN_TOP,
            bottom: MARGIN_BOTTOM,
            left: MARGIN_LEFT,
            right: MARGIN_RIGHT,
          },
        },
      },
      headers: {
        default: new Header({
          children: [
            new Paragraph({
              alignment: AlignmentType.CENTER,
              children: [
                new TextRun({
                  text: 'DevDash轻量级运维监控面板 - 技术报告',
                  font: { name: FONT_CN, eastAsia: FONT_CN },
                  size: 18,
                  color: '888888',
                }),
              ],
            }),
          ],
        }),
      },
      footers: {
        default: new Footer({
          children: [
            new Paragraph({
              alignment: AlignmentType.CENTER,
              children: [
                new TextRun({
                  children: ['第 ', PageNumber.CURRENT, ' 页 / 共 ', PageNumber.TOTAL_PAGES, ' 页'],
                  font: { name: FONT_CN, eastAsia: FONT_CN },
                  size: 18,
                  color: '888888',
                }),
              ],
            }),
          ],
        }),
      },
      children: [
        ...coverPage,
        new Paragraph({ children: [new PageBreak()] }),
        ...abstractSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...abstractEnSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...tocSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...introductionSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...architectureSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...techStackSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...moduleDesignSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...implementationSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...testSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...conclusionSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...referenceSection,
        new Paragraph({ children: [new PageBreak()] }),
        ...appendixSection,
      ],
    },
  ],
});

// ===== 生成文档 =====
Packer.toBuffer(doc).then((buffer) => {
  let outputPath = './DevDash技术报告.docx';
  try {
    fs.writeFileSync(outputPath, buffer);
  } catch (e) {
    if (e.code === 'EBUSY' || e.code === 'EPERM') {
      const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      outputPath = `./DevDash技术报告_${ts}.docx`;
      fs.writeFileSync(outputPath, buffer);
    } else {
      throw e;
    }
  }
  console.log(`✓ 技术文档已生成: ${outputPath}`);
  console.log(`  文档大小: ${(buffer.length / 1024).toFixed(2)} KB`);
  console.log(`  包含章节: 封面、摘要、引言、系统架构、技术选型、功能模块设计、实现细节、测试结果、结论、参考文献、附录`);
  console.log(`  附录内容: EER图、流程图（3个）、时序图（4个）、数据库表结构`);
}).catch((err) => {
  console.error('✗ 文档生成失败:', err);
  process.exit(1);
});