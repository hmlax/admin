import { useTranslation } from 'react-i18next';
import type { ProColumns } from '@ant-design/pro-components';
import type { MuskdeerSyntheticRecord } from '@/api/generated/admin/v1';
import { muskdeerReaders } from '@/api/hooks/muskdeer';
import ResourceTable from '../components/ResourceTable';
export default function Page() {
 const { t } = useTranslation('muskdeer');
 const columns: ProColumns<MuskdeerSyntheticRecord>[] = [
{ title: t('symbol'), dataIndex: 'symbol', hideInTable: true },
{ title: t('state'), dataIndex: 'state', hideInTable: true },
{ title: t('runId'), dataIndex: 'runId', hideInTable: true },
{ title: t('instanceId'), dataIndex: 'instanceId', hideInTable: true },
{ title: t('id'), dataIndex: 'id', hideInSearch: true, width: 160 },
{ title: t('createdAtMs'), dataIndex: 'createdAtMs', hideInSearch: true, width: 160, render: (_, row) => row.createdAtMs ? new Date(row.createdAtMs).toLocaleString() : '—' },
{ title: t('instanceId'), dataIndex: 'instanceId', hideInSearch: true, width: 160 },
{ title: t('runId'), dataIndex: 'runId', hideInSearch: true, width: 160 },
{ title: t('symbol'), dataIndex: 'symbol', hideInSearch: true, width: 160 },
{ title: t('sequence'), dataIndex: 'sequence', hideInSearch: true, width: 160 },
{ title: t('side'), dataIndex: 'side', hideInSearch: true, width: 160 },
{ title: t('price'), dataIndex: 'price', hideInSearch: true, width: 160 },
{ title: t('quantity'), dataIndex: 'quantity', hideInSearch: true, width: 160 },
{ title: t('notional'), dataIndex: 'notional', hideInSearch: true, width: 160 },
{ title: t('state'), dataIndex: 'state', hideInSearch: true, width: 160 },
{ title: t('tradeId'), dataIndex: 'tradeId', hideInSearch: true, width: 160 },
{ title: t('sourceTradeId'), dataIndex: 'sourceTradeId', hideInSearch: true, width: 160 },
];
 return <ResourceTable<MuskdeerSyntheticRecord> resource="synthetic" columns={columns} fetch={muskdeerReaders.synthetic} notice={t('syntheticNotice')} />;
}
