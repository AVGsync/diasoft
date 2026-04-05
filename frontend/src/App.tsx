import {
  Alert,
  App as AntApp,
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
  Upload,
} from "antd";
import {
  BookOutlined,
  CheckCircleOutlined,
  CopyOutlined,
  FileExcelOutlined,
  HomeOutlined,
  KeyOutlined,
  LockOutlined,
  LogoutOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SearchOutlined,
  SendOutlined,
  TeamOutlined,
  UserOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { useEffect, useState, type ReactNode } from "react";
import { Link, Navigate, Route, Routes, useParams, useSearchParams } from "react-router-dom";
import { apiRequest, clearSession, downloadBlob, loadSession, saveSession } from "./lib/api";
import type {
  AdminStats,
  ApiKeyCreateResponse,
  ApiKeySummary,
  Batch,
  BatchUploadResponse,
  DiplomaRecordInput,
  Session,
  ShareLinkResponse,
  SharedDiplomaResponse,
  SigningKeyStatus,
  StudentSearchResult,
  UniversityRecord,
  VerificationByNumberResponse,
  VerificationByPayloadResponse,
  VerificationGeoPoint,
  VerificationStatsResponse,
  VerificationStatusCount,
  VerificationTimeBucket,
  VerificationTopUniversity,
} from "./lib/types";

const { Title, Paragraph, Text } = Typography;
const DEGREE_OPTIONS = ["Бакалавр", "Магистр", "Специалист"];

const demoAccounts = [
  {
    role: "Администратор",
    email: "admin@platform.local",
    password: "Admin12345!",
    note: "Подтверждает заявки вузов и следит за общим реестром.",
  },
  {
    role: "Вуз",
    email: "demo.vuz@platform.local",
    password: "University123!",
    note: "Аккаунт уже активирован и готов к загрузке дипломов.",
  },
];

const emptyRecord = (): DiplomaRecordInput => ({
  full_name: "",
  diploma_number: "",
  specialty: "",
  degree: "Бакалавр",
  faculty: "",
  year: new Date().getFullYear(),
});

type StatsRange = {
  from: string;
  to: string;
};

function toDateInputValue(date: Date) {
  return date.toISOString().slice(0, 10);
}

function defaultStatsRange(): StatsRange {
  const to = new Date();
  const from = new Date();
  from.setDate(to.getDate() - 30);

  return {
    from: toDateInputValue(from),
    to: toDateInputValue(to),
  };
}

function buildStatsRangeQuery(range: StatsRange) {
  const params = new URLSearchParams();
  if (range.from) {
    params.set("from", range.from);
  }
  if (range.to) {
    params.set("to", range.to);
  }

  const query = params.toString();
  return query ? `?${query}` : "";
}

const antTheme = {
  token: {
    colorPrimary: "#374785",
    colorInfo: "#374785",
    colorSuccess: "#374785",
    colorWarning: "#f8e9a1",
    colorError: "#f76c6c",
    colorLink: "#374785",
    colorTextBase: "#24305e",
    colorText: "#24305e",
    colorTextSecondary: "rgba(36, 48, 94, 0.72)",
    colorBgBase: "#fdfcf8",
    colorBgLayout: "#fdfcf8",
    colorBgContainer: "rgba(255, 255, 255, 0.9)",
    colorBorder: "rgba(55, 71, 133, 0.16)",
    colorBorderSecondary: "rgba(55, 71, 133, 0.12)",
    borderRadius: 20,
    borderRadiusLG: 24,
    fontFamily: '"Manrope", "Segoe UI", sans-serif',
    boxShadowSecondary: "0 24px 64px rgba(36, 48, 94, 0.12)",
    motion: false,
  },
  components: {
    Button: {
      controlHeight: 44,
      fontWeight: 600,
      primaryShadow: "none",
    },
    Card: {
      headerFontSize: 18,
    },
    Input: {
      activeBorderColor: "#374785",
      hoverBorderColor: "#374785",
    },
    Select: {
      optionSelectedBg: "rgba(168, 208, 230, 0.24)",
    },
    Tabs: {
      itemActiveColor: "#24305e",
      itemColor: "rgba(36, 48, 94, 0.68)",
      itemHoverColor: "#374785",
      itemSelectedColor: "#24305e",
      inkBarColor: "#f76c6c",
    },
  },
} as const;

export default function App() {
  const [session, setSession] = useState<Session | null>(() => loadSession());

  const handleLogin = (nextSession: Session) => {
    saveSession(nextSession);
    setSession(nextSession);
  };

  const handleLogout = () => {
    clearSession();
    setSession(null);
  };

  return (
    <ConfigProvider theme={antTheme}>
      <AntApp>
        <Routes>
          <Route path="/" element={<MarketingHomePage session={session} />} />
          <Route path="/docs" element={<DocumentationPage session={session} />} />
          <Route path="/login" element={<LoginPage session={session} onLogin={handleLogin} />} />
          <Route path="/check" element={<PublicVerificationPage session={session} />} />
          <Route path="/student" element={<StudentPortalPage session={session} />} />
          <Route path="/verify" element={<VerifyPayloadPage session={session} />} />
          <Route path="/share/:token" element={<SharedDiplomaPage session={session} />} />
          <Route path="/workspace" element={<DashboardPage session={session} onLogout={handleLogout} />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AntApp>
    </ConfigProvider>
  );
}

function DashboardPage({ session, onLogout }: { session: Session | null; onLogout: () => void }) {
  if (!session) {
    return <Navigate to="/login" replace />;
  }

  return (
    <div className="shell-grid min-h-screen px-4 py-6 sm:px-6 lg:px-10">
      <div className="mx-auto max-w-7xl">
        {session.role === "admin" ? (
          <AdminDashboard session={session} onLogout={onLogout} />
        ) : (
          <UniversityDashboard session={session} onLogout={onLogout} />
        )}
      </div>
    </div>
  );
}

function HomePage({ session }: { session: Session | null }) {
  return (
    <PublicPageShell session={session}>
      <div className="grid gap-6 lg:grid-cols-[1.15fr_0.85fr]">
        <Card className="hero-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={24} className="flex">
            <div className="max-w-3xl">
              <Text className="mono-kicker text-moss">единый реестр дипломов</Text>
              <Title level={1} className="!mb-4 !mt-4 !text-[42px] !leading-[1.04] !text-ink sm:!text-[56px]">
                Проверка дипломов, личные кабинеты вузов и публичный доступ для студентов.
              </Title>
              <Paragraph className="!mb-0 !max-w-2xl !text-base !leading-8 !text-slate-700">
                QR-код не вводится вручную. После сканирования система открывает отдельную страницу проверки, а на
                главной оставлены только ключевые действия.
              </Paragraph>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <Link to={session ? "/workspace" : "/login"} className="block">
                <Button type="primary" size="large" icon={<LockOutlined />} className="!h-14 !w-full">
                  {session ? "Открыть кабинет" : "Войти"}
                </Button>
              </Link>
              <Link to="/check" className="block">
                <Button size="large" icon={<SearchOutlined />} className="!h-14 !w-full">
                  Проверить по коду вуза и диплому
                </Button>
              </Link>
              <Link to="/student" className="block">
                <Button size="large" icon={<SafetyCertificateOutlined />} className="!h-14 !w-full">
                  Портал студента
                </Button>
              </Link>
            </div>

            <Alert
              showIcon
              type="info"
              message="Сканирование QR ведёт напрямую на страницу проверки."
              description="Пользователь сканирует код и попадает на маршрут /verify с payload в ссылке. Ручной ввод JWT на главной убран."
            />
          </Space>
        </Card>

        <Space direction="vertical" size={24} className="flex">
          <Card className="accent-card shadow-quiet" bodyStyle={{ padding: 28 }}>
            <Space direction="vertical" size={16} className="flex">
              <Text className="mono-kicker text-moss">для вузов</Text>
              <Title level={3} className="!mb-0 !text-ink">
                Подключение и доступ к кабинету вынесены в отдельный экран.
              </Title>
              <Paragraph className="!mb-0 !text-base !leading-7 !text-slate-700">
                Университет подаёт заявку на регистрацию, а после активации загружает дипломы, выпускает API-ключи и
                управляет публичными документами.
              </Paragraph>
              <Link to="/login">
                <Button type="primary" icon={<TeamOutlined />}>
                  Открыть вход и регистрацию
                </Button>
              </Link>
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 28 }}>
            <Space direction="vertical" size={16} className="flex">
              <Text className="mono-kicker text-moss">публичные сценарии</Text>
              <div className="rounded-[24px] border border-white/80 bg-white/70 p-5">
                <Title level={4} className="!mb-2 !text-ink">
                  Студент может:
                </Title>
                <Paragraph className="!mb-0 !text-base !leading-7 !text-slate-700">
                  проверить свой диплом, выпустить новую QR-метку и получить share-ссылку для работодателя без входа в
                  систему.
                </Paragraph>
              </div>
              <div className="rounded-[24px] border border-[#374785]/12 bg-[#a8d0e6]/18 p-5">
                <Title level={5} className="!mb-2 !text-ink">
                  Проверка для работодателя
                </Title>
                <Paragraph className="!mb-0 !leading-7 !text-slate-700">
                  Работодатель открывает публичную share-ссылку или просто сканирует QR-код диплома.
                </Paragraph>
              </div>
            </Space>
          </Card>
        </Space>
      </div>
    </PublicPageShell>
  );
}

function MarketingHomePage({ session }: { session: Session | null }) {
  return (
    <PublicPageShell session={session}>
      <div className="grid gap-6 lg:grid-cols-[1.08fr_0.92fr]">
        <Card className="hero-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={24} className="flex">
            <div className="max-w-3xl">
              <Text className="mono-kicker text-moss">Единый реестр дипломов</Text>
              <Title level={1} className="!mb-4 !mt-4 !text-[42px] !leading-[1.04] !text-ink sm:!text-[56px]">
                Проверка дипломов, личные кабинеты вузов и публичный доступ для студентов.
              </Title>
              <Paragraph className="!mb-0 !max-w-2xl !text-base !leading-8 !text-slate-700">
                Платформа объединяет выпуск дипломов, ERP-интеграцию, публичную проверку и share-сценарии для
                работодателей в одном интерфейсе.
              </Paragraph>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <Link to={session ? "/workspace" : "/login"} className="block">
                <Button type="primary" size="large" icon={<LockOutlined />} className="!h-14 !w-full">
                  {session ? "Личный кабинет" : "Войти"}
                </Button>
              </Link>
              <Link to="/check" className="block">
                <Button size="large" icon={<SearchOutlined />} className="!h-14 !w-full">
                  Проверить диплом
                </Button>
              </Link>
              <Link to="/student" className="block">
                <Button size="large" icon={<SafetyCertificateOutlined />} className="!h-14 !w-full">
                  Портал студента
                </Button>
              </Link>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <div className="rounded-[22px] border border-[#374785]/12 bg-white/72 p-5">
                <Text className="mono-kicker !text-[11px] !text-moss">ERP</Text>
                <Title level={4} className="!mb-1 !mt-3 !text-ink">
                  API-ключи и batch-загрузка
                </Title>
                <Paragraph className="!mb-0 !text-slate-700">
                  Интеграция с ERP через защищённые ключи, JSON и CSV форматы.
                </Paragraph>
              </div>
              <div className="rounded-[22px] border border-[#374785]/12 bg-white/72 p-5">
                <Text className="mono-kicker !text-[11px] !text-moss">VERIFY</Text>
                <Title level={4} className="!mb-1 !mt-3 !text-ink">
                  Публичная проверка
                </Title>
                <Paragraph className="!mb-0 !text-slate-700">
                  Проверка диплома по номеру, share-ссылке и QR-маршруту.
                </Paragraph>
              </div>
              <div className="rounded-[22px] border border-[#374785]/12 bg-white/72 p-5">
                <Text className="mono-kicker !text-[11px] !text-moss">SECURITY</Text>
                <Title level={4} className="!mb-1 !mt-3 !text-ink">
                  Подпись и хэширование
                </Title>
                <Paragraph className="!mb-0 !text-slate-700">
                  Ed25519 для QR и канонический SHA-256 для реестра дипломов.
                </Paragraph>
              </div>
            </div>
          </Space>
        </Card>

        <Space direction="vertical" size={24} className="flex">
          <Card className="accent-card shadow-quiet" bodyStyle={{ padding: 28 }}>
            <Space direction="vertical" size={16} className="flex">
              <Text className="mono-kicker text-moss">Для вузов</Text>
              <Title level={3} className="!mb-0 !text-ink">
                Документация и подключение ERP собраны в отдельном разделе.
              </Title>
              <Paragraph className="!mb-0 !text-base !leading-7 !text-slate-700">
                Вуз может открыть пошаговую интеграцию, примеры payload, описание ключей, хэшей, публичных ручек и
                полного batch-workflow для ERP.
              </Paragraph>
              <Space wrap>
                <Link to="/docs">
                  <Button type="primary" icon={<BookOutlined />}>
                    Открыть документацию
                  </Button>
                </Link>
                <Link to={session ? "/workspace" : "/login"}>
                  <Button icon={<TeamOutlined />}>
                    {session ? "Перейти в кабинет" : "Вход и регистрация"}
                  </Button>
                </Link>
              </Space>
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 28 }}>
            <Space direction="vertical" size={16} className="flex">
              <Text className="mono-kicker text-moss">Публичные сценарии</Text>
              <div className="rounded-[24px] border border-white/80 bg-white/70 p-5">
                <Title level={4} className="!mb-2 !text-ink">
                  Что доступно сразу
                </Title>
                <Paragraph className="!mb-0 !text-base !leading-7 !text-slate-700">
                  Публичная проверка по коду вуза и номеру диплома, портал студента, share-ссылки и новые QR-коды.
                </Paragraph>
              </div>
              <div className="rounded-[24px] border border-[#374785]/12 bg-[#a8d0e6]/18 p-5">
                <Title level={5} className="!mb-2 !text-ink">
                  Полезно для интеграции
                </Title>
                <Paragraph className="!mb-0 !leading-7 !text-slate-700">
                  Во вкладке документации собраны HTTP API, форматы JSON/CSV, настройка signing key, публичные ручки и
                  ERP-workflow вуза.
                </Paragraph>
              </div>
            </Space>
          </Card>
        </Space>
      </div>
    </PublicPageShell>
  );
}

function DocumentationPage({ session }: { session: Session | null }) {
  const erpEndpoints = [
    { key: "register", method: "POST", path: "/api/v1/auth/register", auth: "Публично", purpose: "Заявка вуза на подключение" },
    { key: "login", method: "POST", path: "/api/v1/auth/login", auth: "Публично", purpose: "Вход администратора и вуза" },
    { key: "signing-key", method: "PUT", path: "/api/v1/vuz/signing-key", auth: "Bearer <jwt>", purpose: "Загрузка Ed25519 private key" },
    { key: "api-key", method: "POST", path: "/api/v1/vuz/api-keys", auth: "Bearer <jwt>", purpose: "Выпуск API-ключа для ERP" },
    { key: "upload", method: "POST", path: "/api/v1/diplomas/upload", auth: "Bearer <jwt> или ApiKey <key>", purpose: "Загрузка дипломов батчем" },
    { key: "batch", method: "GET", path: "/api/v1/diplomas/batches/{batch_id}", auth: "Bearer <jwt> или ApiKey <key>", purpose: "Проверка статуса batch" },
    { key: "download", method: "GET", path: "/api/v1/diplomas/batches/{batch_id}/download", auth: "Bearer <jwt>", purpose: "Выгрузка Excel с QR" },
  ];

  const publicEndpoints = [
    { key: "search", method: "GET", path: "/api/v1/student/search", purpose: "Поиск диплома по номеру и/или ФИО" },
    { key: "share", method: "POST", path: "/api/v1/student/share", purpose: "Генерация share-ссылки для работодателя" },
    { key: "qr", method: "GET", path: "/api/v1/student/qr", purpose: "Повторная генерация QR-кода" },
    { key: "token", method: "GET", path: "/api/v1/student/share/{token}", purpose: "Разворачивание share token" },
    { key: "verify-number", method: "GET", path: "/api/v1/verify/search", purpose: "Публичная проверка по коду вуза и номеру" },
    { key: "verify-qr", method: "GET", path: "/api/v1/verify?payload=<jwt>", purpose: "Публичная проверка по QR payload" },
  ];

  return (
    <PublicPageShell session={session}>
      <Space direction="vertical" size={24} className="flex">
        <Card className="hero-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={20} className="flex">
            <Text className="mono-kicker text-moss">Документация</Text>
            <Title level={1} className="!mb-0 !text-[34px] !leading-tight !text-ink sm:!text-[44px]">
              Интеграция платформы с ERP вуза
            </Title>
            <Paragraph className="!mb-0 !max-w-4xl !text-base !leading-8 !text-slate-700">
              Здесь собраны архитектура проекта, шаги подключения вуза, схемы безопасности, форматы дипломов, batch
              workflow и публичные ручки для студентов и работодателей.
            </Paragraph>
            <Space wrap>
              <Link to={session ? "/workspace" : "/login"}>
                <Button type="primary" icon={<LockOutlined />}>
                  {session ? "Открыть личный кабинет" : "Войти в кабинет"}
                </Button>
              </Link>
              <Link to="/check">
                <Button icon={<SearchOutlined />}>Публичная проверка</Button>
              </Link>
            </Space>
          </Space>
        </Card>

        <Row gutter={[16, 16]}>
          <MetricCard title="Основной API" value="Gateway Service" />
          <MetricCard title="Обработка" value="Kafka + Rust worker" />
          <MetricCard title="Подпись QR" value="Ed25519" />
          <MetricCard title="Хэш диплома" value="SHA-256" />
        </Row>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card className="glass-card shadow-quiet" title="Архитектура платформы">
            <Space direction="vertical" size={14} className="flex">
              <Paragraph className="!mb-0">
                <Text strong>Gateway Service:</Text> основной HTTP API для вузов, администраторов, студентов и ERP.
              </Paragraph>
              <Paragraph className="!mb-0">
                <Text strong>Processing Engine:</Text> воркер без внешнего HTTP API, читает задачи из Kafka и подписывает QR.
              </Paragraph>
              <Paragraph className="!mb-0">
                <Text strong>PostgreSQL:</Text> хранит вузы, API-ключи, batch, результаты, хэши дипломов и signing key.
              </Paragraph>
              <Paragraph className="!mb-0">
                <Text strong>Kafka:</Text> транспорт между gateway и processing engine через `diplomas.raw_tasks` и
                `diplomas.processing_results`.
              </Paragraph>
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" title="Порядок подключения вуза">
            <Space direction="vertical" size={10} className="flex">
              <Paragraph className="!mb-0">1. Вуз отправляет заявку через `POST /api/v1/auth/register`.</Paragraph>
              <Paragraph className="!mb-0">2. Администратор активирует вуз в кабинете платформы.</Paragraph>
              <Paragraph className="!mb-0">3. Вуз логинится через `POST /api/v1/auth/login` и получает access JWT.</Paragraph>
              <Paragraph className="!mb-0">4. Вуз загружает `Ed25519 private key` через `PUT /api/v1/vuz/signing-key`.</Paragraph>
              <Paragraph className="!mb-0">5. Вуз выпускает API-ключ для ERP через `POST /api/v1/vuz/api-keys`.</Paragraph>
              <Paragraph className="!mb-0">6. ERP загружает дипломы батчами и отслеживает их обработку.</Paragraph>
            </Space>
          </Card>
        </div>

        <Card className="glass-card shadow-quiet" title="HTTP API для ERP">
          <Table
            rowKey="key"
            pagination={false}
            dataSource={erpEndpoints}
            columns={[
              { title: "Метод", dataIndex: "method", render: (value: string) => <Tag color="blue">{value}</Tag> },
              { title: "Путь", dataIndex: "path", render: (value: string) => <Text code>{value}</Text> },
              { title: "Авторизация", dataIndex: "auth" },
              { title: "Назначение", dataIndex: "purpose" },
            ]}
          />
        </Card>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card className="glass-card shadow-quiet" title="JSON для загрузки дипломов">
            <CodeBlock
              code={`{
  "diplomas": [
    {
      "full_name": "Иванов Иван Иванович",
      "diploma_number": "DVS-2026-0001",
      "specialty": "Программная инженерия",
      "degree": "Бакалавр",
      "faculty": "Факультет информационных технологий",
      "year": 2026
    }
  ]
}`}
            />
            <Paragraph className="!mb-0 !mt-4 !text-slate-700">
              Допустимые значения `degree`: `Бакалавр`, `Магистр`, `Специалист`.
            </Paragraph>
          </Card>

          <Card className="glass-card shadow-quiet" title="CSV и batch workflow">
            <Space direction="vertical" size={12} className="flex">
              <CodeBlock code={`full_name,diploma_number,specialty,degree,faculty,year`} />
              <Paragraph className="!mb-0 !text-slate-700">
                После загрузки сервис создаёт `batch_id`, публикует задачи в Kafka и возвращает статус `processing`.
              </Paragraph>
              <Paragraph className="!mb-0 !text-slate-700">
                После завершения вуз скачивает Excel по <Text code>GET /api/v1/diplomas/batches/{"{batch_id}"}/download</Text>.
              </Paragraph>
            </Space>
          </Card>
        </div>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card className="glass-card shadow-quiet" title="Signing key и безопасность">
            <Space direction="vertical" size={12} className="flex">
              <Paragraph className="!mb-0 !text-slate-700">
                QR подписывается `Ed25519`, а private key вуза хранится в PostgreSQL в шифрованном виде через
                `AES-256-GCM`.
              </Paragraph>
              <Paragraph className="!mb-0 !text-slate-700">
                Private key не передаётся через Kafka. Воркер читает его из `university_signing_keys` по `vuz_id`.
              </Paragraph>
              <CodeBlock
                code={`{
  "private_key_pem": "-----BEGIN PRIVATE KEY-----\\n...\\n-----END PRIVATE KEY-----"
}`}
              />
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" title="Формат хэша диплома">
            <Space direction="vertical" size={12} className="flex">
              <Paragraph className="!mb-0 !text-slate-700">
                Каноническая строка для расчёта хэша:
              </Paragraph>
              <CodeBlock code={`diploma_number|full_name|specialty|degree|faculty|year|vuz_id|salt`} />
              <Paragraph className="!mb-0 !text-slate-700">
                Алгоритм: `SHA-256`. QR payload затем подписывается `EdDSA / Ed25519`.
              </Paragraph>
            </Space>
          </Card>
        </div>

        <Card className="glass-card shadow-quiet" title="Публичные ручки для студентов и работодателей">
          <Table
            rowKey="key"
            pagination={false}
            dataSource={publicEndpoints}
            columns={[
              { title: "Метод", dataIndex: "method", render: (value: string) => <Tag color="geekblue">{value}</Tag> },
              { title: "Путь", dataIndex: "path", render: (value: string) => <Text code>{value}</Text> },
              { title: "Назначение", dataIndex: "purpose" },
            ]}
          />
        </Card>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card className="glass-card shadow-quiet" title="JWT и конфигурация">
            <Space direction="vertical" size={10} className="flex">
              <Paragraph className="!mb-0">`JWT_SECRET` и `SHARE_JWT_SECRET` используются для access и share token.</Paragraph>
              <Paragraph className="!mb-0">`SIGNING_KEYS_MASTER_KEY` должен быть base64 и после декодирования давать 32 байта.</Paragraph>
              <Paragraph className="!mb-0">`KAFKA_RAW_TOPIC` и `KAFKA_RESULTS_TOPIC` должны совпадать у gateway и worker.</Paragraph>
              <Paragraph className="!mb-0">`PUBLIC_BASE_URL` используется для share-ссылок и QR.</Paragraph>
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" title="Что делает ERP после интеграции">
            <Space direction="vertical" size={10} className="flex">
              <Paragraph className="!mb-0">1. Хранит API-ключ и отправляет batch с дипломами.</Paragraph>
              <Paragraph className="!mb-0">2. Получает `batch_id` и опрашивает статус обработки.</Paragraph>
              <Paragraph className="!mb-0">3. Выгружает Excel с QR и передаёт документы студентам.</Paragraph>
              <Paragraph className="!mb-0">4. При необходимости инициирует повторную выгрузку, отзыв диплома или поиск в реестре.</Paragraph>
            </Space>
          </Card>
        </div>
      </Space>
    </PublicPageShell>
  );
}

function CodeBlock({ code }: { code: string }) {
  return (
    <pre className="doc-code">
      <code>{code}</code>
    </pre>
  );
}

function LoginPage({
  session,
  onLogin,
}: {
  session: Session | null;
  onLogin: (session: Session) => void;
}) {
  const { message } = AntApp.useApp();
  const [loginLoading, setLoginLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);

  if (session) {
    return <Navigate to="/workspace" replace />;
  }

  const handleLogin = async (values: { email: string; password: string }) => {
    setLoginLoading(true);
    try {
      const nextSession = await apiRequest<Session>("/auth/login", {
        method: "POST",
        body: values,
      });

      message.success("Вход выполнен.");
      onLogin(nextSession);
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setLoginLoading(false);
    }
  };

  const handleRegister = async (values: Record<string, string>) => {
    setRegisterLoading(true);
    try {
      await apiRequest("/auth/register", { method: "POST", body: values });
      message.success("Заявка создана. После одобрения администратора можно войти.");
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setRegisterLoading(false);
    }
  };

  return (
    <PublicPageShell session={session}>
      <div className="grid gap-6 lg:grid-cols-[0.98fr_1.02fr]">
        <Card className="accent-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={24} className="flex">
            <div>
              <Text className="mono-kicker text-moss">доступ к системе</Text>
              <Title level={1} className="!mb-4 !mt-4 !text-[38px] !leading-[1.08] !text-ink sm:!text-[48px]">
                Вход для администратора и кабинета вуза.
              </Title>
              <Paragraph className="!mb-0 !max-w-xl !text-base !leading-8 !text-slate-700">
                Здесь университет оставляет заявку на подключение, а администратор и подтверждённый вуз входят в
                рабочие кабинеты. Публичная проверка дипломов и студентский портал вынесены отдельно.
              </Paragraph>
            </div>

            <Card className="!border-0 !bg-[#24305e] !text-white">
              <Space direction="vertical" size={14} className="flex">
                <Text className="mono-kicker !text-[#f8e9a1]">тестовый доступ</Text>
                <Space direction="vertical" size={12} className="flex">
                  {demoAccounts.map((account) => (
                    <div
                      key={account.email}
                      className="flex items-start justify-between gap-4 rounded-2xl border border-white/12 bg-white/8 px-4 py-4"
                    >
                      <div>
                        <Text className="!block !text-sm !text-[#a8d0e6]">{account.role}</Text>
                        <Title level={4} className="!mb-1 !mt-1 !text-white">
                          {account.email}
                        </Title>
                        <Text className="!text-white/75">{account.note}</Text>
                      </div>
                      <div className="text-right">
                        <Text className="mono-kicker !text-xs !text-[#f8e9a1]">Пароль</Text>
                        <div className="mt-2 rounded-xl border border-white/10 bg-black/15 px-3 py-2 text-sm text-white">
                          {account.password}
                        </div>
                      </div>
                    </div>
                  ))}
                </Space>
                <Alert
                  type="info"
                  showIcon
                  message="Код демо-вуза для публичной проверки: DEMO2026"
                  className="!border-none !bg-white/10 !text-white"
                />
              </Space>
            </Card>
          </Space>
        </Card>

        <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Tabs
            animated={false}
            items={[
              {
                key: "login",
                label: "Вход",
                children: (
                  <Form layout="vertical" onFinish={handleLogin}>
                    <Form.Item label="Email" name="email" rules={[{ required: true, type: "email" }]}>
                      <Input prefix={<UserOutlined />} placeholder="admin@platform.local" />
                    </Form.Item>
                    <Form.Item label="Пароль" name="password" rules={[{ required: true }]}>
                      <Input.Password prefix={<LockOutlined />} placeholder="Введите пароль" />
                    </Form.Item>
                    <Button block type="primary" htmlType="submit" loading={loginLoading} icon={<SendOutlined />}>
                      Войти
                    </Button>
                  </Form>
                ),
              },
              {
                key: "register",
                label: "Заявка вуза",
                children: (
                  <Form layout="vertical" onFinish={handleRegister}>
                    <Form.Item label="Название вуза" name="name" rules={[{ required: true }]}>
                      <Input placeholder="МГТУ имени Баумана" />
                    </Form.Item>
                    <Form.Item label="Код вуза" name="vuz_code" rules={[{ required: true, len: 8 }]}>
                      <Input placeholder="BAUMAN26" maxLength={8} />
                    </Form.Item>
                    <Row gutter={12}>
                      <Col span={12}>
                        <Form.Item label="ИНН" name="inn" rules={[{ required: true }]}>
                          <Input placeholder="7701234567" />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item label="ОГРН" name="ogrn" rules={[{ required: true }]}>
                          <Input placeholder="1027700123456" />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Form.Item label="Email" name="email" rules={[{ required: true, type: "email" }]}>
                      <Input placeholder="rector@university.ru" />
                    </Form.Item>
                    <Form.Item label="Пароль" name="password" rules={[{ required: true, min: 8 }]}>
                      <Input.Password placeholder="Не менее 8 символов" />
                    </Form.Item>
                    <Button block type="primary" htmlType="submit" loading={registerLoading} icon={<TeamOutlined />}>
                      Отправить заявку
                    </Button>
                  </Form>
                ),
              },
            ]}
          />
        </Card>
      </div>
    </PublicPageShell>
  );
}

function PublicVerificationPage({ session }: { session: Session | null }) {
  const { message } = AntApp.useApp();
  const [searchResult, setSearchResult] = useState<VerificationByNumberResponse | null>(null);
  const [verifyLoading, setVerifyLoading] = useState(false);

  const handleVerifyByNumber = async (values: { vuz_code: string; diploma_number: string }) => {
    setVerifyLoading(true);
    try {
      const params = new URLSearchParams(values);
      const result = await apiRequest<VerificationByNumberResponse>(`/verify/search?${params.toString()}`, {
        verifyApi: true,
      });

      setSearchResult(result);
    } catch (error) {
      setSearchResult(null);
      message.error((error as Error).message);
    } finally {
      setVerifyLoading(false);
    }
  };

  return (
    <PublicPageShell session={session}>
      <div className="grid gap-6 lg:grid-cols-[0.94fr_1.06fr]">
        <Card className="hero-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={20} className="flex">
            <div>
              <Text className="mono-kicker text-moss">публичная проверка</Text>
              <Title level={2} className="!mb-3 !mt-4 !text-ink">
                Проверка по коду вуза и номеру диплома
              </Title>
              <Paragraph className="!mb-0 !text-base !leading-8 !text-slate-700">
                Используйте код вуза и номер документа. Для QR-проверки ничего вручную вводить не нужно: сканирование
                сразу открывает страницу с результатом.
              </Paragraph>
            </div>

            <Form layout="vertical" onFinish={handleVerifyByNumber}>
              <Form.Item label="Код вуза" name="vuz_code" rules={[{ required: true }]}>
                <Input placeholder="DEMO2026" />
              </Form.Item>
              <Form.Item label="Номер диплома" name="diploma_number" rules={[{ required: true }]}>
                <Input placeholder="DVS-2026-0001" />
              </Form.Item>
              <Button type="primary" icon={<SearchOutlined />} htmlType="submit" loading={verifyLoading}>
                Проверить
              </Button>
            </Form>
          </Space>
        </Card>

        <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={18} className="flex">
            <div>
              <Text className="mono-kicker text-moss">результат</Text>
              <Title level={3} className="!mb-0 !mt-3 !text-ink">
                Статус документа
              </Title>
            </div>
            {searchResult ? (
              <VerificationSearchResult result={searchResult} />
            ) : (
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description="Введите код вуза и номер диплома, чтобы получить публичный статус."
              />
            )}
          </Space>
        </Card>
      </div>
    </PublicPageShell>
  );
}

function StudentPortalPage({ session }: { session: Session | null }) {
  const { message } = AntApp.useApp();
  const [students, setStudents] = useState<StudentSearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [shareLink, setShareLink] = useState<ShareLinkResponse | null>(null);

  const handleSearch = async (values: { diploma_number?: string; full_name?: string }) => {
    const diplomaNumber = values.diploma_number?.trim() ?? "";
    const fullName = values.full_name?.trim() ?? "";

    if (!diplomaNumber && !fullName) {
      message.warning("Укажите номер диплома или ФИО.");
      return;
    }

    setLoading(true);
    try {
      const params = new URLSearchParams();
      if (diplomaNumber) {
        params.set("diploma_number", diplomaNumber);
      }
      if (fullName) {
        params.set("full_name", fullName);
      }

      const response = await apiRequest<StudentSearchResult[]>(`/student/search?${params.toString()}`);
      setStudents(response);
      if (response.length === 0) {
        message.info("Совпадений не найдено.");
      }
    } catch (error) {
      setStudents([]);
      message.error((error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const createShareLink = async (record: StudentSearchResult) => {
    try {
      const response = await apiRequest<ShareLinkResponse>("/student/share", {
        method: "POST",
        body: { diploma_hash: record.diploma_hash, ttl_hours: 72 },
      });

      setShareLink(response);
      message.success("Share-ссылка создана.");
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const downloadQr = async (record: StudentSearchResult) => {
    try {
      const blob = await apiRequest<Blob>(
        `/student/qr?diploma_hash=${encodeURIComponent(record.diploma_hash)}&format=png&ttl_hours=72`,
        { responseType: "blob" },
      );
      downloadBlob(blob, `${record.diploma_number}.png`);
      message.success("QR-код сформирован.");
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const studentColumns: ColumnsType<StudentSearchResult> = [
    {
      title: "Студент",
      dataIndex: "full_name",
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.full_name}</Text>
          <Text type="secondary">{record.university_name}</Text>
        </Space>
      ),
    },
    {
      title: "Диплом",
      dataIndex: "diploma_number",
    },
    {
      title: "Программа",
      render: (_, record) => `${record.specialty} · ${record.year}`,
    },
    {
      title: "Статус",
      dataIndex: "status",
      render: (value) => <StatusTag value={value} />,
    },
    {
      title: "Действия",
      render: (_, record) => (
        <Space wrap>
          <Button onClick={() => void createShareLink(record)}>Поделиться</Button>
          <Button icon={<ReloadOutlined />} onClick={() => void downloadQr(record)}>
            Сгенерировать QR
          </Button>
        </Space>
      ),
    },
  ];

  const applyStatsRange = () => {
    setAppliedStatsRange(draftStatsRange);
  };

  const resetStatsRange = () => {
    const nextRange = defaultStatsRange();
    setDraftStatsRange(nextRange);
    setAppliedStatsRange(nextRange);
  };

  return (
    <PublicPageShell session={session}>
      <Space direction="vertical" size={24} className="flex">
        <div className="grid gap-6 lg:grid-cols-[0.92fr_1.08fr]">
          <Card className="hero-card shadow-quiet" bodyStyle={{ padding: 32 }}>
            <Space direction="vertical" size={20} className="flex">
              <div>
                <Text className="mono-kicker text-moss">портал студента</Text>
                <Title level={2} className="!mb-3 !mt-4 !text-ink">
                  Найдите диплом, создайте share-ссылку и получите новый QR.
                </Title>
                <Paragraph className="!mb-0 !text-base !leading-8 !text-slate-700">
                  Портал работает без авторизации. После поиска студент может подтвердить диплом, открыть ссылку для
                  работодателя и повторно скачать QR-код.
                </Paragraph>
              </div>

              <Form layout="vertical" onFinish={handleSearch}>
                <Form.Item label="Номер диплома" name="diploma_number">
                  <Input placeholder="DVS-2026-0001" />
                </Form.Item>
                <Form.Item label="ФИО" name="full_name">
                  <Input placeholder="Иванов Иван Иванович" />
                </Form.Item>
                <Button type="primary" icon={<SearchOutlined />} htmlType="submit" loading={loading}>
                  Найти диплом
                </Button>
              </Form>
            </Space>
          </Card>

          <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 32 }}>
            <Space direction="vertical" size={18} className="flex">
              <div>
                <Text className="mono-kicker text-moss">результаты поиска</Text>
                <Title level={3} className="!mb-0 !mt-3 !text-ink">
                  Публичные записи
                </Title>
              </div>

              {students.length > 0 ? (
                <Table
                  rowKey="diploma_hash"
                  loading={loading}
                  columns={studentColumns}
                  dataSource={students}
                  pagination={{ pageSize: 5 }}
                />
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="После поиска здесь появятся записи, доступные для share-ссылки и генерации QR."
                />
              )}
            </Space>
          </Card>
        </div>

        {shareLink && (
          <Alert
            type="success"
            showIcon
            message="Ссылка для работодателя"
            description={
              <Space direction="vertical" size={8}>
                <Text strong>{shareLink.share_url}</Text>
                <Text type="secondary">Действует до {formatDate(shareLink.expires_at)}</Text>
                <Space wrap>
                  <Button
                    icon={<CopyOutlined />}
                    onClick={async () => {
                      if (navigator.clipboard) {
                        await navigator.clipboard.writeText(shareLink.share_url);
                        message.success("Ссылка скопирована.");
                      }
                    }}
                  >
                    Скопировать
                  </Button>
                  <a href={shareLink.share_url} target="_blank" rel="noreferrer">
                    <Button type="primary">Открыть ссылку</Button>
                  </a>
                </Space>
              </Space>
            }
          />
        )}
      </Space>
    </PublicPageShell>
  );
}

function PublicPageShell({
  session,
  children,
}: {
  session: Session | null;
  children: ReactNode;
}) {
  return (
    <div className="shell-grid min-h-screen px-4 py-6 sm:px-6 lg:px-10">
      <div className="mx-auto max-w-6xl">
        <PortalTopBar session={session} />
        {children}
      </div>
    </div>
  );
}

function PortalTopBar({ session }: { session: Session | null }) {
  return (
    <div className="mb-6 flex flex-col gap-4 rounded-[28px] border border-white/80 bg-white/85 px-5 py-4 shadow-quiet backdrop-blur md:flex-row md:items-center md:justify-between">
      <Link to="/" className="brand-link">
        <span className="brand-badge">DR</span>
        <span>
          <Text className="mono-kicker !block !text-[11px] !text-moss">digital diploma registry</Text>
          <Title level={4} className="!mb-0 !mt-1 !text-ink">
            Реестр дипломов
          </Title>
        </span>
      </Link>

      <Space wrap size="middle">
        <Link to="/docs">
          <Button type="text">Документация</Button>
        </Link>
        <Link to="/check">
          <Button type="text">Проверка диплома</Button>
        </Link>
        <Link to="/student">
          <Button type="text">Портал студента</Button>
        </Link>
        <Link to={session ? "/workspace" : "/login"}>
          <Button type="primary" icon={<LockOutlined />}>
            {session ? "Личный кабинет" : "Войти"}
          </Button>
        </Link>
      </Space>
    </div>
  );
}

function LandingPage({ onLogin }: { onLogin: (session: Session) => void }) {
  const { message } = AntApp.useApp();
  const [loginLoading, setLoginLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);
  const [searchResult, setSearchResult] = useState<VerificationByNumberResponse | null>(null);
  const [payloadResult, setPayloadResult] = useState<VerificationByPayloadResponse | null>(null);
  const [verifyLoading, setVerifyLoading] = useState(false);

  const handleLogin = async (values: { email: string; password: string }) => {
    setLoginLoading(true);
    try {
      const session = await apiRequest<Session>("/auth/login", {
        method: "POST",
        body: values,
      });

      message.success("Вход выполнен.");
      onLogin(session);
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setLoginLoading(false);
    }
  };

  const handleRegister = async (values: Record<string, string>) => {
    setRegisterLoading(true);
    try {
      await apiRequest("/auth/register", { method: "POST", body: values });
      message.success("Заявка создана. После одобрения администратора можно войти.");
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setRegisterLoading(false);
    }
  };

  const handleVerifyByNumber = async (values: { vuz_code: string; diploma_number: string }) => {
    setVerifyLoading(true);
    try {
      const params = new URLSearchParams(values);
      const result = await apiRequest<VerificationByNumberResponse>(`/verify/search?${params.toString()}`, {
        verifyApi: true,
      });

      setSearchResult(result);
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setVerifyLoading(false);
    }
  };

  const handleVerifyByPayload = async (values: { payload: string }) => {
    setVerifyLoading(true);
    try {
      const params = new URLSearchParams({ payload: values.payload.trim() });
      const result = await apiRequest<VerificationByPayloadResponse>(`/verify?${params.toString()}`, {
        verifyApi: true,
      });

      setPayloadResult(result);
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setVerifyLoading(false);
    }
  };

  return (
    <Space direction="vertical" size={24} className="flex">
      <div className="grid gap-6 lg:grid-cols-[1.2fr_0.9fr]">
        <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 28 }}>
          <Space direction="vertical" size={24} className="flex">
            <Text className="mono-kicker text-moss">Diasoft Hackathon Stack</Text>
            <div className="max-w-2xl">
              <Title level={1} className="!mb-3 !text-[42px] !leading-[1.04] !text-ink sm:!text-[56px]">
                Единая витрина для регистрации вузов, выпуска дипломов и публичной проверки.
              </Title>
              <Paragraph className="!mb-0 !max-w-xl !text-base !leading-7 !text-slate-700">
                Фронтенд работает поверх `gateway-service` и `pubver`. Сайт уже знает тестовые роли, умеет
                загружать дипломы, выпускать API-ключи и выдавать share-link для студента.
              </Paragraph>
            </div>

            <Row gutter={[16, 16]}>
              <MetricCard title="Backend" value="Go + Rust" />
              <MetricCard title="Public verify" value="Pubver" />
              <MetricCard title="Transport" value="Kafka" />
              <MetricCard title="Deploy" value="Docker Compose" />
            </Row>

            <Card className="!border-0 !bg-[#122321] !text-white">
              <Space direction="vertical" size={14} className="flex">
                <Text className="mono-kicker !text-sage">Тестовые учётки</Text>
                <List
                  split={false}
                  dataSource={demoAccounts}
                  renderItem={(account) => (
                    <List.Item className="!px-0">
                      <div className="flex w-full items-start justify-between gap-4 rounded-2xl border border-white/10 bg-white/5 px-4 py-4">
                        <div>
                          <Text className="!block !text-sm !text-sage">{account.role}</Text>
                          <Title level={4} className="!mb-1 !mt-1 !text-white">
                            {account.email}
                          </Title>
                          <Text className="!text-white/75">{account.note}</Text>
                        </div>
                        <div className="text-right">
                          <Text className="mono-kicker !text-xs !text-sage">Пароль</Text>
                          <div className="mt-2 rounded-xl border border-white/10 bg-black/20 px-3 py-2 text-sm text-white">
                            {account.password}
                          </div>
                        </div>
                      </div>
                    </List.Item>
                  )}
                />
                <Alert type="info" showIcon message="Код демо-вуза для публичного поиска: DEMO2026" />
              </Space>
            </Card>
          </Space>
        </Card>

        <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 28 }}>
          <Tabs
            animated={false}
            items={[
              {
                key: "login",
                label: "Вход",
                children: (
                  <Form layout="vertical" onFinish={handleLogin}>
                    <Form.Item label="Email" name="email" rules={[{ required: true, type: "email" }]}>
                      <Input prefix={<UserOutlined />} placeholder="admin@platform.local" />
                    </Form.Item>
                    <Form.Item label="Пароль" name="password" rules={[{ required: true }]}>
                      <Input.Password prefix={<LockOutlined />} placeholder="Введите пароль" />
                    </Form.Item>
                    <Button block type="primary" htmlType="submit" loading={loginLoading} icon={<SendOutlined />}>
                      Войти
                    </Button>
                  </Form>
                ),
              },
              {
                key: "register",
                label: "Заявка вуза",
                children: (
                  <Form layout="vertical" onFinish={handleRegister}>
                    <Form.Item label="Название вуза" name="name" rules={[{ required: true }]}>
                      <Input placeholder="МГТУ имени Баумана" />
                    </Form.Item>
                    <Form.Item label="Код вуза" name="vuz_code" rules={[{ required: true, len: 8 }]}>
                      <Input placeholder="BAUMAN26" maxLength={8} />
                    </Form.Item>
                    <Row gutter={12}>
                      <Col span={12}>
                        <Form.Item label="ИНН" name="inn" rules={[{ required: true }]}>
                          <Input placeholder="7701234567" />
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item label="ОГРН" name="ogrn" rules={[{ required: true }]}>
                          <Input placeholder="1027700123456" />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Form.Item label="Email" name="email" rules={[{ required: true, type: "email" }]}>
                      <Input placeholder="rector@university.ru" />
                    </Form.Item>
                    <Form.Item label="Пароль" name="password" rules={[{ required: true, min: 8 }]}>
                      <Input.Password placeholder="Не менее 8 символов" />
                    </Form.Item>
                    <Button block type="primary" htmlType="submit" loading={registerLoading} icon={<TeamOutlined />}>
                      Отправить заявку
                    </Button>
                  </Form>
                ),
              },
            ]}
          />

          <Divider />

          <Tabs
            animated={false}
            size="small"
            items={[
              {
                key: "search",
                label: "Проверка по номеру",
                children: (
                  <Space direction="vertical" size={16} className="flex">
                    <Form layout="vertical" onFinish={handleVerifyByNumber}>
                      <Row gutter={12}>
                        <Col span={10}>
                          <Form.Item label="Код вуза" name="vuz_code" rules={[{ required: true }]}>
                            <Input placeholder="DEMO2026" />
                          </Form.Item>
                        </Col>
                        <Col span={14}>
                          <Form.Item label="Номер диплома" name="diploma_number" rules={[{ required: true }]}>
                            <Input placeholder="DVS-2026-0001" />
                          </Form.Item>
                        </Col>
                      </Row>
                      <Button icon={<SearchOutlined />} htmlType="submit" loading={verifyLoading}>
                        Проверить
                      </Button>
                    </Form>
                    {searchResult && <VerificationSearchResult result={searchResult} />}
                  </Space>
                ),
              },
              {
                key: "payload",
                label: "Проверка по QR payload",
                children: (
                  <Space direction="vertical" size={16} className="flex">
                    <Form layout="vertical" onFinish={handleVerifyByPayload}>
                      <Form.Item label="JWT из QR" name="payload" rules={[{ required: true }]}>
                        <Input.TextArea rows={6} placeholder="Вставьте qr_payload" />
                      </Form.Item>
                      <Button icon={<SafetyCertificateOutlined />} htmlType="submit" loading={verifyLoading}>
                        Проверить payload
                      </Button>
                    </Form>
                    {payloadResult && <VerificationPayloadResult result={payloadResult} />}
                  </Space>
                ),
              },
            ]}
          />
        </Card>
      </div>
    </Space>
  );
}

function AdminDashboard({ session, onLogout }: { session: Session; onLogout: () => void }) {
  const { message } = AntApp.useApp();
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [verificationStats, setVerificationStats] = useState<VerificationStatsResponse | null>(null);
  const [universities, setUniversities] = useState<UniversityRecord[]>([]);
  const [appliedStatsRange, setAppliedStatsRange] = useState<StatsRange>(() => defaultStatsRange());
  const [draftStatsRange, setDraftStatsRange] = useState<StatsRange>(() => defaultStatsRange());
  const [loading, setLoading] = useState(false);
  const [actionId, setActionId] = useState<string | null>(null);

  const loadData = async () => {
    setLoading(true);
    try {
      const [nextStats, nextUniversities, nextVerificationStats] = await Promise.all([
        apiRequest<AdminStats>("/admin/stats", { token: session.access_token }),
        apiRequest<UniversityRecord[]>("/admin/universities", { token: session.access_token }),
        apiRequest<VerificationStatsResponse>(`/admin/stats/verifications${buildStatsRangeQuery(appliedStatsRange)}`, {
          token: session.access_token,
        }),
      ]);

      setStats(nextStats);
      setUniversities(nextUniversities);
      setVerificationStats(normalizeVerificationStats(nextVerificationStats));
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, [appliedStatsRange.from, appliedStatsRange.to]);

  const updateStatus = async (record: UniversityRecord, status: string) => {
    setActionId(record.id);
    try {
      if (status === "active" && record.status === "pending") {
        await apiRequest(`/admin/universities/${record.id}/approve`, {
          method: "POST",
          token: session.access_token,
        });
      } else {
        await apiRequest(`/admin/universities/${record.id}`, {
          method: "PATCH",
          token: session.access_token,
          body: { status },
        });
      }

      message.success("Статус обновлён.");
      await loadData();
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setActionId(null);
    }
  };

  const columns: ColumnsType<UniversityRecord> = [
    {
      title: "Вуз",
      dataIndex: "name",
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.name}</Text>
          <Text type="secondary">{record.email}</Text>
        </Space>
      ),
    },
    {
      title: "Код",
      dataIndex: "vuz_code",
    },
    {
      title: "Статус",
      dataIndex: "status",
      render: (value) => <StatusTag value={value} />,
    },
    {
      title: "Ключ",
      dataIndex: "has_public_key",
      render: (value: boolean) => (value ? <Tag color="green">готов</Tag> : <Tag>нет</Tag>),
    },
    {
      title: "Действие",
      key: "actions",
      render: (_, record) => (
        <Select
          value={record.status}
          style={{ width: 160 }}
          loading={actionId === record.id}
          options={[
            { label: "pending", value: "pending" },
            { label: "active", value: "active" },
            { label: "blocked", value: "blocked" },
          ]}
          onChange={(value) => void updateStatus(record, value)}
        />
      ),
    },
  ];

  const applyStatsRange = () => {
    setAppliedStatsRange(draftStatsRange);
  };

  const resetStatsRange = () => {
    const nextRange = defaultStatsRange();
    setDraftStatsRange(nextRange);
    setAppliedStatsRange(nextRange);
  };

  return (
    <Space direction="vertical" size={20} className="flex">
      <HeaderCard
        title="Кабинет администратора"
        subtitle={`Вход выполнен как ${session.email}`}
        actions={
          <Space>
            <Link to="/">
              <Button icon={<HomeOutlined />}>Главная</Button>
            </Link>
            <Link to="/docs">
              <Button icon={<BookOutlined />}>Документация</Button>
            </Link>
            <Button icon={<ReloadOutlined />} onClick={() => void loadData()} loading={loading}>
              Обновить
            </Button>
            <Button icon={<LogoutOutlined />} onClick={onLogout}>
              Выйти
            </Button>
          </Space>
        }
      />

      <Row gutter={[16, 16]}>
        <MetricCard title="Все вузы" value={stats?.total_universities ?? 0} />
        <MetricCard title="Ожидают одобрения" value={stats?.pending_universities ?? 0} />
        <MetricCard title="Активные" value={stats?.active_universities ?? 0} />
        <MetricCard title="Отозванные дипломы" value={stats?.revoked_diplomas ?? 0} />
      </Row>

      <Card className="glass-card shadow-quiet" title="Реестр вузов">
        <Table rowKey="id" loading={loading} columns={columns} dataSource={universities} pagination={{ pageSize: 8 }} />
      </Card>

      <StatsRangeControls
        draftRange={draftStatsRange}
        onDraftChange={setDraftStatsRange}
        onApply={applyStatsRange}
        onReset={resetStatsRange}
        loading={loading}
      />

      <VerificationStatsPanel
        title="Аналитика проверок по платформе"
        stats={verificationStats}
        loading={loading}
        showTopUniversities
      />
    </Space>
  );
}

function UniversityDashboard({ session, onLogout }: { session: Session; onLogout: () => void }) {
  const { message } = AntApp.useApp();
  const [profile, setProfile] = useState<UniversityRecord | null>(null);
  const [signingKeyStatus, setSigningKeyStatus] = useState<SigningKeyStatus | null>(null);
  const [verificationStats, setVerificationStats] = useState<VerificationStatsResponse | null>(null);
  const [batches, setBatches] = useState<Batch[]>([]);
  const [apiKeys, setApiKeys] = useState<ApiKeySummary[]>([]);
  const [latestApiKey, setLatestApiKey] = useState<ApiKeyCreateResponse | null>(null);
  const [shareLink, setShareLink] = useState<ShareLinkResponse | null>(null);
  const [students, setStudents] = useState<StudentSearchResult[]>([]);
  const [appliedStatsRange, setAppliedStatsRange] = useState<StatsRange>(() => defaultStatsRange());
  const [draftStatsRange, setDraftStatsRange] = useState<StatsRange>(() => defaultStatsRange());
  const [loading, setLoading] = useState(false);
  const [csvFile, setCsvFile] = useState<File | null>(null);

  const loadCabinet = async () => {
    setLoading(true);
    try {
      const [nextProfile, nextSigningKey, nextBatches, nextKeys, nextVerificationStats] = await Promise.all([
        apiRequest<UniversityRecord>("/vuz/profile", { token: session.access_token }),
        apiRequest<SigningKeyStatus>("/vuz/signing-key", { token: session.access_token }).catch(() => null),
        apiRequest<Batch[]>("/vuz/batches?limit=10", { token: session.access_token }),
        apiRequest<ApiKeySummary[]>("/vuz/api-keys", { token: session.access_token }),
        apiRequest<VerificationStatsResponse>(`/vuz/stats/verifications${buildStatsRangeQuery(appliedStatsRange)}`, {
          token: session.access_token,
        }),
      ]);

      setProfile(nextProfile);
      setSigningKeyStatus(nextSigningKey);
      setBatches(nextBatches);
      setApiKeys(nextKeys);
      setVerificationStats(normalizeVerificationStats(nextVerificationStats));
    } catch (error) {
      message.error((error as Error).message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadCabinet();
  }, [appliedStatsRange.from, appliedStatsRange.to]);

  const uploadSigningKey = async (values: { private_key_pem: string }) => {
    try {
      const response = await apiRequest<SigningKeyStatus>("/vuz/signing-key", {
        method: "PUT",
        token: session.access_token,
        body: values,
      });

      setSigningKeyStatus(response);
      message.success("Ключ подписи сохранён.");
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const createApiKey = async (values: { name?: string }) => {
    try {
      const response = await apiRequest<ApiKeyCreateResponse>("/vuz/api-keys", {
        method: "POST",
        token: session.access_token,
        body: { name: values.name || null },
      });

      setLatestApiKey(response);
      message.success("API-ключ выпущен. Сохраните его сейчас.");
      await loadCabinet();
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const uploadDiplomas = async (values: { diplomas: DiplomaRecordInput[] }) => {
    try {
      const payload = {
        diplomas: values.diplomas
          .filter((item) => item.full_name.trim() !== "")
          .map((item) => ({
            ...item,
            year: Number(item.year),
          })),
      };

      const response = await apiRequest<BatchUploadResponse>("/diplomas/upload", {
        method: "POST",
        token: session.access_token,
        body: payload,
      });

      message.success(`Батч ${response.batch_id} отправлен в обработку.`);
      await loadCabinet();
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const uploadCsv = async () => {
    if (!csvFile) {
      message.warning("Сначала выберите CSV-файл.");
      return;
    }

    const formData = new FormData();
    formData.append("file", csvFile);

    try {
      const response = await apiRequest<BatchUploadResponse>("/diplomas/upload", {
        method: "POST",
        token: session.access_token,
        body: formData,
      });

      message.success(`CSV загружен. Батч ${response.batch_id} создан.`);
      setCsvFile(null);
      await loadCabinet();
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const searchStudents = async (values: { diploma_number?: string; full_name?: string }) => {
    try {
      const params = new URLSearchParams();
      if (values.diploma_number?.trim()) {
        params.set("diploma_number", values.diploma_number.trim());
      }
      if (values.full_name?.trim()) {
        params.set("full_name", values.full_name.trim());
      }

      const response = await apiRequest<StudentSearchResult[]>(`/student/search?${params.toString()}`);
      setStudents(response);
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const createShareLink = async (record: StudentSearchResult) => {
    try {
      const response = await apiRequest<ShareLinkResponse>("/student/share", {
        method: "POST",
        body: { diploma_hash: record.diploma_hash, ttl_hours: 72 },
      });

      setShareLink(response);
      message.success("Share-link создан.");
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const downloadStudentQr = async (record: StudentSearchResult) => {
    try {
      const blob = await apiRequest<Blob>(
        `/student/qr?diploma_hash=${encodeURIComponent(record.diploma_hash)}&format=png&ttl_hours=72`,
        { responseType: "blob" },
      );
      downloadBlob(blob, `${record.diploma_number}.png`);
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const revokeDiploma = async (record: StudentSearchResult) => {
    Modal.confirm({
      title: "Отозвать диплом?",
      content: `Будет отозван диплом ${record.diploma_number}.`,
      okText: "Отозвать",
      cancelText: "Отмена",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await apiRequest(`/diplomas/${record.diploma_hash}/revoke`, {
            method: "PATCH",
            token: session.access_token,
          });

          message.success("Диплом отозван.");
          await searchStudents({ diploma_number: record.diploma_number });
          await loadCabinet();
        } catch (error) {
          message.error((error as Error).message);
        }
      },
    });
  };

  const downloadBatch = async (batch: Batch) => {
    try {
      const blob = await apiRequest<Blob>(`/diplomas/batches/${batch.id}/download`, {
        token: session.access_token,
        responseType: "blob",
      });

      downloadBlob(blob, `batch_${batch.id}.xlsx`);
    } catch (error) {
      message.error((error as Error).message);
    }
  };

  const batchColumns: ColumnsType<Batch> = [
    {
      title: "Батч",
      dataIndex: "id",
      render: (value: string) => <Text copyable={{ text: value }}>{value.slice(0, 8)}...</Text>,
    },
    {
      title: "Статус",
      dataIndex: "status",
      render: (value) => <StatusTag value={value} />,
    },
    {
      title: "Прогресс",
      render: (_, record) => `${record.processed_records}/${record.total_records}`,
    },
    {
      title: "Ошибки",
      dataIndex: "failed_records",
    },
    {
      title: "Действие",
      render: (_, record) => (
        <Button icon={<FileExcelOutlined />} onClick={() => void downloadBatch(record)}>
          Excel
        </Button>
      ),
    },
  ];

  const studentColumns: ColumnsType<StudentSearchResult> = [
    { title: "ФИО", dataIndex: "full_name" },
    { title: "Диплом", dataIndex: "diploma_number" },
    { title: "Статус", dataIndex: "status", render: (value) => <StatusTag value={value} /> },
    {
      title: "Действия",
      render: (_, record) => (
        <Space wrap>
          <Button onClick={() => void createShareLink(record)}>Share</Button>
          <Button onClick={() => void downloadStudentQr(record)}>QR</Button>
          <Button danger onClick={() => void revokeDiploma(record)}>
            Revoke
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size={20} className="flex">
      <HeaderCard
        title="Кабинет вуза"
        subtitle={profile ? `${profile.name} · ${profile.vuz_code}` : session.email}
        actions={
          <Space>
            <Link to="/">
              <Button icon={<HomeOutlined />}>Главная</Button>
            </Link>
            <Link to="/docs">
              <Button icon={<BookOutlined />}>Документация</Button>
            </Link>
            <Button icon={<ReloadOutlined />} onClick={() => void loadCabinet()} loading={loading}>
              Обновить
            </Button>
            <Button icon={<LogoutOutlined />} onClick={onLogout}>
              Выйти
            </Button>
          </Space>
        }
      />

      {latestApiKey && (
        <Alert
          type="success"
          showIcon
          message="Новый API-ключ"
          description={
            <Space direction="vertical">
              <Text strong>{latestApiKey.api_key}</Text>
              <Button
                icon={<CopyOutlined />}
                onClick={async () => {
                  if (navigator.clipboard) {
                    await navigator.clipboard.writeText(latestApiKey.api_key);
                  }
                }}
              >
                Скопировать
              </Button>
            </Space>
          }
        />
      )}

      {shareLink && (
        <Alert
          type="info"
          showIcon
          message="Share-link для диплома"
          description={
            <Space direction="vertical">
              <Text strong>{shareLink.share_url}</Text>
              <Text type="secondary">Действует до {formatDate(shareLink.expires_at)}</Text>
            </Space>
          }
        />
      )}

      <Tabs
        animated={false}
        items={[
          {
            key: "overview",
            label: "Обзор",
            children: (
              <Space direction="vertical" size={16} className="flex">
                <Row gutter={[16, 16]}>
                  <MetricCard title="Последних батчей" value={batches.length} />
                  <MetricCard title="API-ключей" value={apiKeys.length} />
                  <MetricCard title="Статус ключа" value={signingKeyStatus?.configured ? "OK" : "Нет"} />
                  <MetricCard title="Email" value={profile?.email ?? session.email} />
                </Row>

                <Row gutter={[16, 16]}>
                  <Col xs={24} lg={12}>
                    <Card className="glass-card shadow-quiet" title="Профиль вуза">
                      {profile ? (
                        <Descriptions column={1} size="small">
                          <Descriptions.Item label="Название">{profile.name}</Descriptions.Item>
                          <Descriptions.Item label="Код">{profile.vuz_code}</Descriptions.Item>
                          <Descriptions.Item label="Статус">
                            <StatusTag value={profile.status} />
                          </Descriptions.Item>
                          <Descriptions.Item label="Публичный ключ">
                            {profile.has_public_key ? "Настроен" : "Отсутствует"}
                          </Descriptions.Item>
                        </Descriptions>
                      ) : (
                        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Профиль загружается" />
                      )}
                    </Card>
                  </Col>
                  <Col xs={24} lg={12}>
                    <Card className="glass-card shadow-quiet" title="Ключ подписи">
                      {signingKeyStatus?.configured ? (
                        <Descriptions column={1} size="small">
                          <Descriptions.Item label="Алгоритм">{signingKeyStatus.key_algorithm}</Descriptions.Item>
                          <Descriptions.Item label="Шифрование">
                            {signingKeyStatus.encryption_algorithm}
                          </Descriptions.Item>
                          <Descriptions.Item label="Fingerprint">
                            <Text copyable={{ text: signingKeyStatus.public_key_fingerprint }}>
                              {signingKeyStatus.public_key_fingerprint.slice(0, 16)}...
                            </Text>
                          </Descriptions.Item>
                        </Descriptions>
                      ) : (
                        <Alert
                          type="warning"
                          showIcon
                          message="Ключ подписи ещё не загружен"
                          description="Без него крипто-воркер не сможет выпустить QR payload."
                        />
                      )}
                    </Card>
                  </Col>
                </Row>

                <Card className="glass-card shadow-quiet" title="Последние батчи">
                  <Table rowKey="id" columns={batchColumns} dataSource={batches} pagination={false} />
                </Card>
              </Space>
            ),
          },
          {
            key: "upload",
            label: "Загрузка дипломов",
            children: (
              <Space direction="vertical" size={16} className="flex">
                <Card className="glass-card shadow-quiet" title="JSON-форма">
                  <Form layout="vertical" initialValues={{ diplomas: [emptyRecord()] }} onFinish={uploadDiplomas}>
                    <Form.List name="diplomas">
                      {(fields, { add, remove }) => (
                        <Space direction="vertical" size={16} className="flex">
                          {fields.map((field) => (
                            <Card
                              key={field.key}
                              type="inner"
                              title={`Запись #${field.name + 1}`}
                              extra={
                                fields.length > 1 ? (
                                  <Button danger onClick={() => remove(field.name)}>
                                    Удалить
                                  </Button>
                                ) : null
                              }
                            >
                              <Row gutter={12}>
                                <Col xs={24} md={12}>
                                  <Form.Item label="ФИО" name={[field.name, "full_name"]} rules={[{ required: true }]}>
                                    <Input />
                                  </Form.Item>
                                </Col>
                                <Col xs={24} md={12}>
                                  <Form.Item
                                    label="Номер диплома"
                                    name={[field.name, "diploma_number"]}
                                    rules={[{ required: true }]}
                                  >
                                    <Input />
                                  </Form.Item>
                                </Col>
                                <Col xs={24} md={12}>
                                  <Form.Item
                                    label="Специальность"
                                    name={[field.name, "specialty"]}
                                    rules={[{ required: true }]}
                                  >
                                    <Input />
                                  </Form.Item>
                                </Col>
                                <Col xs={24} md={12}>
                                  <Form.Item label="Факультет" name={[field.name, "faculty"]} rules={[{ required: true }]}>
                                    <Input />
                                  </Form.Item>
                                </Col>
                                <Col xs={24} md={12}>
                                  <Form.Item label="Степень" name={[field.name, "degree"]} rules={[{ required: true }]}>
                                    <Select options={DEGREE_OPTIONS.map((item) => ({ label: item, value: item }))} />
                                  </Form.Item>
                                </Col>
                                <Col xs={24} md={12}>
                                  <Form.Item label="Год" name={[field.name, "year"]} rules={[{ required: true }]}>
                                    <InputNumber className="!w-full" min={1900} max={2100} precision={0} />
                                  </Form.Item>
                                </Col>
                              </Row>
                            </Card>
                          ))}
                          <Space>
                            <Button onClick={() => add(emptyRecord())}>Добавить запись</Button>
                            <Button type="primary" htmlType="submit" icon={<SendOutlined />}>
                              Отправить в обработку
                            </Button>
                          </Space>
                        </Space>
                      )}
                    </Form.List>
                  </Form>
                </Card>

                <Card className="glass-card shadow-quiet" title="CSV-загрузка">
                  <Space direction="vertical" size={16} className="flex">
                    <Alert
                      type="info"
                      showIcon
                      message="Формат CSV"
                      description="Заголовки: full_name,diploma_number,specialty,degree,faculty,year"
                    />
                    <Upload.Dragger
                      maxCount={1}
                      beforeUpload={(file) => {
                        setCsvFile(file);
                        return false;
                      }}
                      onRemove={() => {
                        setCsvFile(null);
                      }}
                    >
                      <p className="ant-upload-text">Перетащите CSV или нажмите для выбора файла</p>
                    </Upload.Dragger>
                    <Button type="primary" icon={<FileExcelOutlined />} onClick={() => void uploadCsv()}>
                      Отправить CSV
                    </Button>
                  </Space>
                </Card>
              </Space>
            ),
          },
          {
            key: "security",
            label: "Ключи и API",
            children: (
              <Space direction="vertical" size={16} className="flex">
                <Card className="glass-card shadow-quiet" title="Приватный ключ подписи">
                  <Form layout="vertical" onFinish={uploadSigningKey}>
                    <Form.Item
                      label="Ed25519 private key (PEM)"
                      name="private_key_pem"
                      rules={[{ required: true }]}
                    >
                      <Input.TextArea rows={8} placeholder="-----BEGIN PRIVATE KEY-----" />
                    </Form.Item>
                    <Button htmlType="submit" type="primary" icon={<KeyOutlined />}>
                      Сохранить ключ
                    </Button>
                  </Form>
                </Card>

                <Card className="glass-card shadow-quiet" title="API-ключи для ERP">
                  <Space direction="vertical" size={16} className="flex">
                    <Form layout="inline" onFinish={createApiKey}>
                      <Form.Item name="name">
                        <Input placeholder="Например, ERP integration" />
                      </Form.Item>
                      <Form.Item>
                        <Button type="primary" htmlType="submit">
                          Выпустить ключ
                        </Button>
                      </Form.Item>
                    </Form>
                    <Table
                      rowKey="id"
                      dataSource={apiKeys}
                      pagination={false}
                      columns={[
                        { title: "Имя", dataIndex: "name", render: (value) => value || "Без названия" },
                        {
                          title: "Статус",
                          dataIndex: "is_active",
                          render: (value: boolean) => <StatusTag value={value ? "active" : "blocked"} />,
                        },
                        { title: "Создан", dataIndex: "created_at", render: (value) => formatDate(value) },
                      ]}
                    />
                  </Space>
                </Card>
              </Space>
            ),
          },
          {
            key: "analytics",
            label: "Аналитика",
            children: (
              <Space direction="vertical" size={16} className="flex">
                <StatsRangeControls
                  draftRange={draftStatsRange}
                  onDraftChange={setDraftStatsRange}
                  onApply={applyStatsRange}
                  onReset={resetStatsRange}
                  loading={loading}
                />
              <VerificationStatsPanel
                title="Аналитика проверок дипломов"
                stats={verificationStats}
                loading={loading}
              />
              </Space>
            ),
          },
          {
            key: "registry",
            label: "Поиск и отзыв",
            children: (
              <Space direction="vertical" size={16} className="flex">
                <Card className="glass-card shadow-quiet" title="Поиск по реестру">
                  <Form layout="vertical" onFinish={searchStudents}>
                    <Row gutter={12}>
                      <Col xs={24} md={12}>
                        <Form.Item label="Номер диплома" name="diploma_number">
                          <Input placeholder="DVS-2026-0001" />
                        </Form.Item>
                      </Col>
                      <Col xs={24} md={12}>
                        <Form.Item label="ФИО" name="full_name">
                          <Input placeholder="Иванов Иван Иванович" />
                        </Form.Item>
                      </Col>
                    </Row>
                    <Button htmlType="submit" icon={<SearchOutlined />}>
                      Найти
                    </Button>
                  </Form>
                </Card>

                <Card className="glass-card shadow-quiet" title="Результаты">
                  {students.length > 0 ? (
                    <Table rowKey="diploma_hash" columns={studentColumns} dataSource={students} pagination={false} />
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Сначала выполните поиск." />
                  )}
                </Card>
              </Space>
            ),
          },
        ]}
      />
    </Space>
  );
}

function SharedDiplomaPage({ session }: { session: Session | null }) {
  const { token } = useParams<{ token: string }>();
  const { message } = AntApp.useApp();
  const [payload, setPayload] = useState<SharedDiplomaResponse | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!token) {
      return;
    }

    const run = async () => {
      setLoading(true);
      try {
        const response = await apiRequest<SharedDiplomaResponse>(`/student/share/${token}`);
        setPayload(response);
      } catch (error) {
        message.error((error as Error).message);
      } finally {
        setLoading(false);
      }
    };

    void run();
  }, [token]);

  return (
    <PublicPageShell session={session}>
      <div className="mx-auto max-w-3xl">
        <Card className="glass-card shadow-quiet" loading={loading} bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={20} className="flex">
            <Text className="mono-kicker text-moss">share-ссылка</Text>
            <Title level={2} className="!mb-0">
              {payload ? payload.full_name : "Данные диплома"}
            </Title>
            {payload ? (
              <>
                <Descriptions column={1} bordered>
                  <Descriptions.Item label="Номер диплома">{payload.diploma_number}</Descriptions.Item>
                  <Descriptions.Item label="Специальность">{payload.specialty}</Descriptions.Item>
                  <Descriptions.Item label="Степень">{payload.degree}</Descriptions.Item>
                  <Descriptions.Item label="Факультет">{payload.faculty}</Descriptions.Item>
                  <Descriptions.Item label="Год">{payload.year}</Descriptions.Item>
                  <Descriptions.Item label="Вуз">{payload.university_name}</Descriptions.Item>
                  <Descriptions.Item label="Статус">
                    <StatusTag value={payload.status} />
                  </Descriptions.Item>
                  <Descriptions.Item label="Действует до">{formatDate(payload.expires_at)}</Descriptions.Item>
                </Descriptions>
                <Link to="/">
                  <Button type="primary">Вернуться на главную</Button>
                </Link>
              </>
            ) : (
              <Alert type="warning" showIcon message="Ссылка невалидна или срок действия истёк." />
            )}
          </Space>
        </Card>
      </div>
    </PublicPageShell>
  );
}

function VerifyPayloadPage({ session }: { session: Session | null }) {
  const [searchParams] = useSearchParams();
  const { message } = AntApp.useApp();
  const [result, setResult] = useState<VerificationByPayloadResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [errorText, setErrorText] = useState<string | null>(null);
  const payload = searchParams.get("payload")?.trim() ?? "";

  useEffect(() => {
    if (!payload) {
      setResult(null);
      setErrorText(null);
      return;
    }

    const run = async () => {
      setLoading(true);
      setErrorText(null);

      try {
        const params = new URLSearchParams({ payload });
        const response = await apiRequest<VerificationByPayloadResponse>(`/verify?${params.toString()}`, {
          verifyApi: true,
        });

        setResult(response);
      } catch (error) {
        const nextError = (error as Error).message;
        setResult(null);
        setErrorText(nextError);
        message.error(nextError);
      } finally {
        setLoading(false);
      }
    };

    void run();
  }, [message, payload]);

  return (
    <PublicPageShell session={session}>
      <div className="mx-auto max-w-3xl">
        <Card className="glass-card shadow-quiet" loading={loading} bodyStyle={{ padding: 32 }}>
          <Space direction="vertical" size={20} className="flex">
            <Text className="mono-kicker text-moss">проверка по qr</Text>
            <Title level={2} className="!mb-0">
              Проверка QR payload
            </Title>
            {!payload ? (
              <Alert type="info" showIcon message="В ссылке отсутствует параметр payload." />
            ) : result ? (
              <VerificationPayloadResult result={result} />
            ) : errorText ? (
              <Alert type="error" showIcon message={errorText} />
            ) : (
              <Alert type="warning" showIcon message="Не удалось получить данные для проверки." />
            )}
            <Link to="/">
              <Button type="primary">Вернуться на главную</Button>
            </Link>
          </Space>
        </Card>
      </div>
    </PublicPageShell>
  );
}

function HeaderCard({
  title,
  subtitle,
  actions,
}: {
  title: string;
  subtitle: string;
  actions: ReactNode;
}) {
  return (
    <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 24 }}>
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <Text className="mono-kicker text-moss">Private workspace</Text>
          <Title level={2} className="!mb-1 !mt-1">
            {title}
          </Title>
          <Text type="secondary">{subtitle}</Text>
        </div>
        <div>{actions}</div>
      </div>
    </Card>
  );
}

function MetricCard({ title, value }: { title: string; value: number | string }) {
  return (
    <Col xs={24} sm={12} xl={6}>
      <Card className="glass-card shadow-quiet h-full">
        <Statistic title={title} value={value} />
      </Card>
    </Col>
  );
}

function StatsRangeControls({
  draftRange,
  onDraftChange,
  onApply,
  onReset,
  loading,
}: {
  draftRange: StatsRange;
  onDraftChange: (range: StatsRange) => void;
  onApply: () => void;
  onReset: () => void;
  loading: boolean;
}) {
  return (
    <Card className="glass-card shadow-quiet" bodyStyle={{ padding: 16 }}>
      <Space wrap align="end">
        <div>
          <Text type="secondary">Период с</Text>
          <Input
            type="date"
            value={draftRange.from}
            onChange={(event) => onDraftChange({ ...draftRange, from: event.target.value })}
            style={{ width: 170 }}
          />
        </div>
        <div>
          <Text type="secondary">По</Text>
          <Input
            type="date"
            value={draftRange.to}
            onChange={(event) => onDraftChange({ ...draftRange, to: event.target.value })}
            style={{ width: 170 }}
          />
        </div>
        <Button type="primary" onClick={onApply} loading={loading}>
          Применить
        </Button>
        <Button onClick={onReset} disabled={loading}>
          Последние 30 дней
        </Button>
      </Space>
    </Card>
  );
}

function normalizeVerificationStats(stats: VerificationStatsResponse): VerificationStatsResponse {
  return {
    ...stats,
    statuses: stats.statuses ?? [],
    timeseries: stats.timeseries ?? [],
    geography: stats.geography ?? [],
    top_universities: stats.top_universities ?? [],
  };
}

function getStatusCount(stats: VerificationStatsResponse | null, status: string) {
  if (!stats) {
    return 0;
  }

  return stats.statuses.find((item) => item.status === status)?.count ?? 0;
}

function VerificationStatsPanel({
  title,
  stats,
  loading,
  showTopUniversities = false,
}: {
  title: string;
  stats: VerificationStatsResponse | null;
  loading: boolean;
  showTopUniversities?: boolean;
}) {
  const statusColumns: ColumnsType<VerificationStatusCount> = [
    { title: "Статус", dataIndex: "status" },
    { title: "Проверок", dataIndex: "count" },
  ];

  const timeseriesColumns: ColumnsType<VerificationTimeBucket> = [
    { title: "Дата", dataIndex: "date" },
    { title: "Проверок", dataIndex: "count" },
  ];

  const geographyColumns: ColumnsType<VerificationGeoPoint> = [
    {
      title: "Страна",
      dataIndex: "country",
      render: (value?: string) => value || "Не определена",
    },
    {
      title: "Город",
      dataIndex: "city",
      render: (value?: string) => value || "Не определен",
    },
    { title: "Проверок", dataIndex: "count" },
  ];

  const topUniversitiesColumns: ColumnsType<VerificationTopUniversity> = [
    {
      title: "Вуз",
      render: (_, record) => record.name || record.vuz_code || record.vuz_id || "Не определен",
    },
    {
      title: "Код",
      dataIndex: "vuz_code",
      render: (value?: string) => value || "—",
    },
    { title: "Проверок", dataIndex: "checks" },
  ];

  return (
    <Card className="glass-card shadow-quiet" title={title} loading={loading}>
      {stats ? (
        <Space direction="vertical" size={20} className="flex">
          <Row gutter={[16, 16]}>
            <MetricCard title="Всего проверок" value={stats.total_checks} />
            <MetricCard title="Уникальных источников" value={stats.unique_requesters} />
            <MetricCard title="Active" value={getStatusCount(stats, "active")} />
            <MetricCard title="Invalid payload" value={getStatusCount(stats, "invalid_payload")} />
            <MetricCard title="Not found" value={getStatusCount(stats, "not_found")} />
            <MetricCard title="Revoked" value={getStatusCount(stats, "revoked")} />
          </Row>

          <Descriptions column={1} size="small">
            <Descriptions.Item label="Период с">{formatDate(stats.from)}</Descriptions.Item>
            <Descriptions.Item label="Период по">{formatDate(stats.to)}</Descriptions.Item>
          </Descriptions>

          <Row gutter={[16, 16]}>
            <Col xs={24} lg={12}>
              <Card type="inner" title="Распределение статусов">
                <Table
                  rowKey={(record) => record.status}
                  columns={statusColumns}
                  dataSource={stats.statuses}
                  pagination={false}
                  locale={{ emptyText: "Пока нет данных" }}
                />
              </Card>
            </Col>
            <Col xs={24} lg={12}>
              <Card type="inner" title="Динамика по дням">
                <Table
                  rowKey={(record) => record.date}
                  columns={timeseriesColumns}
                  dataSource={stats.timeseries}
                  pagination={false}
                  locale={{ emptyText: "Пока нет данных" }}
                />
              </Card>
            </Col>
          </Row>

          <Card type="inner" title="География запросов">
            <Table
              rowKey={(record) => `${record.country ?? ""}-${record.city ?? ""}-${record.count}`}
              columns={geographyColumns}
              dataSource={stats.geography}
              pagination={false}
              locale={{ emptyText: "География пока не определена" }}
            />
          </Card>

          {showTopUniversities && (
            <Card type="inner" title="Топ университетов">
              <Table
                rowKey={(record) => record.vuz_id || record.vuz_code || record.name || String(record.checks)}
                columns={topUniversitiesColumns}
                dataSource={stats.top_universities ?? []}
                pagination={false}
                locale={{ emptyText: "Пока нет данных" }}
              />
            </Card>
          )}
        </Space>
      ) : (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Аналитика пока не загружена." />
      )}
    </Card>
  );
}

function StatusTag({ value }: { value: string }) {
  if (value === "active" || value === "completed" || value === "valid") {
    return <Tag color="green">{value}</Tag>;
  }
  if (value === "pending" || value === "processing") {
    return <Tag color="gold">{value}</Tag>;
  }
  if (value === "revoked" || value === "blocked" || value === "failed" || value === "not_found") {
    return <Tag color="red">{value}</Tag>;
  }

  return <Tag>{value}</Tag>;
}

function VerificationSearchResult({ result }: { result: VerificationByNumberResponse }) {
  return (
    <Card type="inner">
      <Space direction="vertical" size={6}>
        <Space>
          <StatusTag value={result.status} />
          <Text strong>{result.valid ? "Диплом найден и активен" : "Диплом не найден или недействителен"}</Text>
        </Space>
        <Text>{result.university || "Вуз не найден"}</Text>
        <Text type="secondary">
          {result.specialty || "Специальность пока отсутствует"} · {result.year || "год не указан"}
        </Text>
      </Space>
    </Card>
  );
}

function VerificationPayloadResult({ result }: { result: VerificationByPayloadResponse }) {
  return (
    <Card type="inner">
      <Space direction="vertical" size={6}>
        <Space>
          {result.valid ? (
            <CheckCircleOutlined className="text-green-600" />
          ) : (
            <SafetyCertificateOutlined className="text-clay" />
          )}
          <Text strong>{result.valid ? "Подпись и хэш совпали" : "Проверка payload не пройдена"}</Text>
        </Space>
        <Text>{result.university || "Вуз не определён"}</Text>
        <Text type="secondary">
          {result.diploma_number || "Номер диплома отсутствует"} · {result.hash || "hash отсутствует"}
        </Text>
      </Space>
    </Card>
  );
}

function formatDate(value?: string | null) {
  if (!value) {
    return "—";
  }

  return new Date(value).toLocaleString("ru-RU");
}
