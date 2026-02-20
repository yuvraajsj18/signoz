import { useCallback, useState } from 'react';
import { useLocation } from 'react-router-dom';
import { Button, Popover } from 'antd';
import logEvent from 'api/common/logEvent';
import { useGetTenantLicense } from 'hooks/useGetTenantLicense';
import { Calculator, Globe, Inbox, SquarePen } from 'lucide-react';

import AnnouncementsModal from './AnnouncementsModal';
import CloudCostEstimatorPopover from './CloudCostEstimatorPopover';
import FeedbackModal from './FeedbackModal';
import ShareURLModal from './ShareURLModal';

import './HeaderRightSection.styles.scss';

interface HeaderRightSectionProps {
	enableAnnouncements: boolean;
	enableShare: boolean;
	enableFeedback: boolean;
}

function HeaderRightSection({
	enableAnnouncements,
	enableShare,
	enableFeedback,
}: HeaderRightSectionProps): JSX.Element | null {
	const location = useLocation();

	const [openFeedbackModal, setOpenFeedbackModal] = useState(false);
	const [openShareURLModal, setOpenShareURLModal] = useState(false);
	const [openAnnouncementsModal, setOpenAnnouncementsModal] = useState(false);
	const [openCloudCostModal, setOpenCloudCostModal] = useState(false);

	const {
		isCloudUser,
		isEnterpriseSelfHostedUser,
		isCommunityUser,
		isCommunityEnterpriseUser,
	} = useGetTenantLicense();

	const handleOpenFeedbackModal = useCallback((): void => {
		logEvent('Feedback: Clicked', {
			page: location.pathname,
		});

		setOpenFeedbackModal(true);
		setOpenShareURLModal(false);
		setOpenAnnouncementsModal(false);
		setOpenCloudCostModal(false);
	}, [location.pathname]);

	const handleOpenShareURLModal = useCallback((): void => {
		logEvent('Share: Clicked', {
			page: location.pathname,
		});

		setOpenShareURLModal(true);
		setOpenFeedbackModal(false);
		setOpenAnnouncementsModal(false);
		setOpenCloudCostModal(false);
	}, [location.pathname]);

	const handleOpenCloudCostModal = useCallback((): void => {
		logEvent('Cloud Cost: Clicked', {
			page: location.pathname,
		});

		setOpenCloudCostModal(true);
		setOpenShareURLModal(false);
		setOpenFeedbackModal(false);
		setOpenAnnouncementsModal(false);
	}, [location.pathname]);

	const handleCloseFeedbackModal = (): void => {
		setOpenFeedbackModal(false);
	};

	const handleOpenFeedbackModalChange = (open: boolean): void => {
		setOpenFeedbackModal(open);
	};

	const handleOpenAnnouncementsModalChange = (open: boolean): void => {
		setOpenAnnouncementsModal(open);
	};

	const handleOpenCloudCostModalChange = (open: boolean): void => {
		setOpenCloudCostModal(open);
	};

	const handleOpenShareURLModalChange = (open: boolean): void => {
		setOpenShareURLModal(open);
	};

	const isLicenseEnabled = isEnterpriseSelfHostedUser || isCloudUser;
	const isOssUser =
		isCommunityUser ||
		isCommunityEnterpriseUser ||
		(!isCloudUser && !isEnterpriseSelfHostedUser);

	return (
		<div className="header-right-section-container">
			{enableFeedback && isLicenseEnabled && (
				<Popover
					rootClassName="header-section-popover-root"
					className="shareable-link-popover"
					placement="bottomRight"
					content={<FeedbackModal onClose={handleCloseFeedbackModal} />}
					destroyTooltipOnHide
					arrow={false}
					trigger="click"
					open={openFeedbackModal}
					onOpenChange={handleOpenFeedbackModalChange}
				>
					<Button
						className="share-feedback-btn periscope-btn ghost"
						icon={<SquarePen size={14} />}
						onClick={handleOpenFeedbackModal}
					>
						Feedback
					</Button>
				</Popover>
			)}

			{enableAnnouncements && (
				<Popover
					rootClassName="header-section-popover-root"
					className="shareable-link-popover"
					placement="bottomRight"
					content={<AnnouncementsModal />}
					arrow={false}
					destroyTooltipOnHide
					trigger="click"
					open={openAnnouncementsModal}
					onOpenChange={handleOpenAnnouncementsModalChange}
				>
					<Button
						icon={<Inbox size={14} />}
						className="periscope-btn ghost announcements-btn"
						onClick={(): void => {
							logEvent('Announcements: Clicked', {
								page: location.pathname,
							});
						}}
					/>
				</Popover>
			)}

			{isOssUser && (
				<Popover
					rootClassName="header-section-popover-root"
					className="shareable-link-popover"
					placement="bottomRight"
					content={<CloudCostEstimatorPopover />}
					open={openCloudCostModal}
					destroyTooltipOnHide
					arrow={false}
					trigger="click"
					onOpenChange={handleOpenCloudCostModalChange}
				>
					<Button
						className="cloud-cost-btn periscope-btn ghost"
						icon={<Calculator size={14} />}
						onClick={handleOpenCloudCostModal}
					>
						Cloud Cost
					</Button>
				</Popover>
			)}

			{enableShare && (
				<Popover
					rootClassName="header-section-popover-root"
					className="shareable-link-popover"
					placement="bottomRight"
					content={<ShareURLModal />}
					open={openShareURLModal}
					destroyTooltipOnHide
					arrow={false}
					trigger="click"
					onOpenChange={handleOpenShareURLModalChange}
				>
					<Button
						className="share-link-btn periscope-btn ghost"
						icon={<Globe size={14} />}
						onClick={handleOpenShareURLModal}
					>
						Share
					</Button>
				</Popover>
			)}
		</div>
	);
}

export default HeaderRightSection;
