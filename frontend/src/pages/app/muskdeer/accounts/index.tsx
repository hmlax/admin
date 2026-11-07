import { useTranslation } from 'react-i18next';
import type { ProColumns } from '@ant-design/pro-components';
import { Tag } from 'antd';
import type { MuskdeerAccount } from '@/api/generated/admin/v1';
import { muskdeerReaders } from '@/api/hooks/muskdeer';
import ResourceTable from '../components/ResourceTable';
export default function Page() {
 const { t } = useTranslation('muskdeer');
 const columns: ProColumns<MuskdeerAccount>[] = [
{ title: t('search'), dataIndex: 'search', hideInTable: true },
{ title: t('accountId'), dataIndex: 'accountId', hideInSearch: true, width: 160 },
{ title: t('symbol'), dataIndex: 'symbol', hideInSearch: true, width: 160 },
{ title: t('userId'), dataIndex: 'userId', hideInSearch: true, width: 160 },
{ title: t('fundId'), dataIndex: 'fundId', hideInSearch: true, width: 160 },
{ title: t('enabled'), dataIndex: 'enabled', hideInSearch: true, width: 160, render: (_, row) => <Tag color={row.enabled ? 'green' : 'default'}>{t(row.enabled ? 'yes' : 'no')}</Tag> },
{ title: t('maximumNotional'), dataIndex: 'maximumNotional', hideInSearch: true, width: 160 },
{ title: t('maximumOrderNotional'), dataIndex: 'maximumOrderNotional', hideInSearch: true, width: 160 },
{ title: t('leverage'), dataIndex: 'leverage', hideInSearch: true, width: 160 },
{ title: t('observedNotional'), dataIndex: 'observedNotional', hideInSearch: true, width: 160 },
{ title: t('reservedNotional'), dataIndex: 'reservedNotional', hideInSearch: true, width: 160 },
{ title: t('availableQuote'), dataIndex: 'availableQuote', hideInSearch: true, width: 160 },
{ title: t('snapshotExpiresAtMs'), dataIndex: 'snapshotExpiresAtMs', hideInSearch: true, width: 160, render: (_, row) => row.snapshotExpiresAtMs ? new Date(row.snapshotExpiresAtMs).toLocaleString() : '—' },
];
 return <ResourceTable<MuskdeerAccount> resource="accounts" columns={columns} fetch={muskdeerReaders.accounts} />;
}
